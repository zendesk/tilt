package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/tilt-dev/tilt/internal/timecmp"

	"github.com/tilt-dev/tilt/internal/store/k8sconv"

	"github.com/tilt-dev/wmclient/pkg/analytics"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	tiltanalytics "github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/hud/view"
	"github.com/tilt-dev/tilt/internal/ospath"
	"github.com/tilt-dev/tilt/internal/token"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

type EngineState struct {
	TiltBuildInfo model.TiltBuild
	TiltStartTime time.Time

	// saved so that we can render in order
	ManifestDefinitionOrder []model.ManifestName

	// TODO(nick): This will eventually be a general Target index.
	ManifestTargets map[model.ManifestName]*ManifestTarget

	CurrentlyBuilding map[model.ManifestName]bool
	EngineMode        EngineMode
	TerminalMode      TerminalMode

	// For synchronizing BuildController -- wait until engine records all builds started
	// so far before starting another build
	StartedBuildCount int

	// How many builds have been completed (pass or fail) since starting tilt
	CompletedBuildCount int

	// For synchronizing ConfigsController -- wait until engine records all builds started
	// so far before starting another build
	StartedTiltfileLoadCount int

	UpdateSettings model.UpdateSettings

	FatalError error

	// The user has indicated they want to exit
	UserExited bool

	// We recovered from a panic(). We need to clean up the RTY and print the error.
	PanicExited error

	// Normal process termination. Either Tilt completed all its work,
	// or it determined that it was unable to complete the work it was assigned.
	//
	// Note that ExitSignal/ExitError is never triggered in normal
	// 'tilt up`/dev mode. It's more for CI modes and tilt up --watch=false modes.
	//
	// We don't provide the ability to customize exit codes. Either the
	// process exited successfully, or with an error. In the future, we might
	// add the ability to put an exit code in the error.
	ExitSignal bool
	ExitError  error

	// All logs in Tilt, stored in a structured format.
	LogStore *logstore.LogStore `testdiff:"ignore"`

	TiltfilePath string

	// TODO(nick): This should be called "ConfigPaths", not "ConfigFiles",
	// because this could refer to directories that are watched recursively.
	ConfigFiles []string

	Tiltignore    model.Dockerignore
	WatchSettings model.WatchSettings

	TriggerQueue []model.ManifestName

	IsProfiling bool

	TiltfileState *ManifestState

	SuggestedTiltVersion string
	VersionSettings      model.VersionSettings

	// Analytics Info
	AnalyticsEnvOpt        analytics.Opt
	AnalyticsUserOpt       analytics.Opt // changes to this field will propagate into the TiltAnalytics subscriber + we'll record them as user choice
	AnalyticsTiltfileOpt   analytics.Opt // Set by the Tiltfile. Overrides the UserOpt.
	AnalyticsNudgeSurfaced bool          // this flag is set the first time we show the analytics nudge to the user.

	Features map[string]bool

	Secrets model.SecretSet

	CloudAddress string
	Token        token.Token
	TeamID       string

	CloudStatus CloudStatus

	DockerPruneSettings model.DockerPruneSettings

	TelemetrySettings model.TelemetrySettings

	MetricsSettings model.MetricsSettings
	MetricsServing  MetricsServing

	UserConfigState model.UserConfigState

	// API-server-based data models. Stored in EngineState
	// to assist in migration.
	Cmds                  map[string]*Cmd                               `json:"-"`
	FileWatches           map[types.NamespacedName]*v1alpha1.FileWatch  `json:"-"`
	KubernetesDiscoveries map[types.NamespacedName]*KubernetesDiscovery `json:"-"`
	PodLogStreams         map[string]*PodLogStream                      `json:"-"`
	PortForwards          map[string]*PortForward                       `json:"-"`
}

type CloudStatus struct {
	Username                         string
	TeamName                         string
	TokenKnownUnregistered           bool // to distinguish whether an empty Username means "we haven't checked" or "we checked and the token isn't registered"
	WaitingForStatusPostRegistration bool
}

// Merge analytics opt-in status from different sources.
// The Tiltfile opt-in takes precedence over the user opt-in.
func (e *EngineState) AnalyticsEffectiveOpt() analytics.Opt {
	if e.AnalyticsEnvOpt != analytics.OptDefault {
		return e.AnalyticsEnvOpt
	}
	if e.AnalyticsTiltfileOpt != analytics.OptDefault {
		return e.AnalyticsTiltfileOpt
	}
	return e.AnalyticsUserOpt
}

func (e *EngineState) ManifestNamesForTargetID(id model.TargetID) []model.ManifestName {
	if id.Type == model.TargetTypeConfigs {
		return []model.ManifestName{model.TiltfileManifestName}
	}

	result := make([]model.ManifestName, 0)
	for mn, mt := range e.ManifestTargets {
		manifest := mt.Manifest
		for _, iTarget := range manifest.ImageTargets {
			if iTarget.ID() == id {
				result = append(result, mn)
			}
		}
		if manifest.K8sTarget().ID() == id {
			result = append(result, mn)
		}
		if manifest.DockerComposeTarget().ID() == id {
			result = append(result, mn)
		}
		if manifest.LocalTarget().ID() == id {
			result = append(result, mn)
		}
	}
	return result
}

func (e *EngineState) IsCurrentlyBuilding(name model.ManifestName) bool {
	return e.CurrentlyBuilding[name]
}

// Find the first build status. Only suitable for testing.
func (e *EngineState) BuildStatus(id model.TargetID) BuildStatus {
	mns := e.ManifestNamesForTargetID(id)
	for _, mn := range mns {
		ms := e.ManifestTargets[mn].State
		bs := ms.BuildStatus(id)
		if !bs.IsEmpty() {
			return bs
		}
	}
	return BuildStatus{}
}

func (e *EngineState) AvailableBuildSlots() int {
	currentlyBuilding := len(e.CurrentlyBuilding)
	if currentlyBuilding >= e.UpdateSettings.MaxParallelUpdates() {
		// this could happen if user decreases max build slots while
		// multiple builds are in progress, no big deal
		return 0
	}
	return e.UpdateSettings.MaxParallelUpdates() - currentlyBuilding
}

func (e *EngineState) UpsertManifestTarget(mt *ManifestTarget) {
	mn := mt.Manifest.Name
	_, ok := e.ManifestTargets[mn]
	if !ok {
		e.ManifestDefinitionOrder = append(e.ManifestDefinitionOrder, mn)
	}
	e.ManifestTargets[mn] = mt
}

func (e *EngineState) RemoveManifestTarget(mn model.ManifestName) {
	delete(e.ManifestTargets, mn)
	newOrder := []model.ManifestName{}
	for _, n := range e.ManifestDefinitionOrder {
		if n == mn {
			continue
		}
		newOrder = append(newOrder, n)
	}
	e.ManifestDefinitionOrder = newOrder
}

func (e EngineState) Manifest(mn model.ManifestName) (model.Manifest, bool) {
	m, ok := e.ManifestTargets[mn]
	if !ok {
		return model.Manifest{}, ok
	}
	return m.Manifest, ok
}

func (e EngineState) ManifestState(mn model.ManifestName) (*ManifestState, bool) {
	if mn == model.TiltfileManifestName {
		return e.TiltfileState, true
	}

	m, ok := e.ManifestTargets[mn]
	if !ok {
		return nil, ok
	}
	return m.State, ok
}

// Returns Manifests in a stable order
func (e EngineState) Manifests() []model.Manifest {
	result := make([]model.Manifest, 0, len(e.ManifestTargets))
	for _, mn := range e.ManifestDefinitionOrder {
		mt, ok := e.ManifestTargets[mn]
		if !ok {
			continue
		}
		result = append(result, mt.Manifest)
	}
	return result
}

// Returns ManifestStates in a stable order
func (e EngineState) ManifestStates() []*ManifestState {
	result := make([]*ManifestState, 0, len(e.ManifestTargets))
	for _, mn := range e.ManifestDefinitionOrder {
		mt, ok := e.ManifestTargets[mn]
		if !ok {
			continue
		}
		result = append(result, mt.State)
	}
	return result
}

// Returns ManifestTargets in a stable order
func (e EngineState) Targets() []*ManifestTarget {
	result := make([]*ManifestTarget, 0, len(e.ManifestTargets))
	for _, mn := range e.ManifestDefinitionOrder {
		mt, ok := e.ManifestTargets[mn]
		if !ok {
			continue
		}
		result = append(result, mt)
	}
	return result
}

func (e EngineState) TargetsBesides(mn model.ManifestName) []*ManifestTarget {
	targets := e.Targets()
	result := make([]*ManifestTarget, 0, len(targets))
	for _, mt := range targets {
		if mt.Manifest.Name == mn {
			continue
		}

		result = append(result, mt)
	}
	return result
}

func (e *EngineState) ManifestInTriggerQueue(mn model.ManifestName) bool {
	for _, queued := range e.TriggerQueue {
		if queued == mn {
			return true
		}
	}
	return false
}

func (e *EngineState) TiltfileInTriggerQueue() bool {
	return e.ManifestInTriggerQueue(model.TiltfileManifestName)
}

func (e *EngineState) AppendToTriggerQueue(mn model.ManifestName, reason model.BuildReason) {
	ms, ok := e.ManifestState(mn)
	if !ok {
		return
	}

	if reason == 0 {
		reason = model.BuildReasonFlagTriggerUnknown
	}

	ms.TriggerReason = ms.TriggerReason.With(reason)

	for _, queued := range e.TriggerQueue {
		if mn == queued {
			return
		}
	}
	e.TriggerQueue = append(e.TriggerQueue, mn)
}

func (e *EngineState) RemoveFromTriggerQueue(mn model.ManifestName) {
	mState, ok := e.ManifestState(mn)
	if ok {
		mState.TriggerReason = model.BuildReasonNone
	}

	for i, triggerName := range e.TriggerQueue {
		if triggerName == mn {
			e.TriggerQueue = append(e.TriggerQueue[:i], e.TriggerQueue[i+1:]...)
			break
		}
	}
}

func (e EngineState) RelativeTiltfilePath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Rel(wd, e.TiltfilePath)
}

func (e EngineState) IsEmpty() bool {
	return len(e.ManifestTargets) == 0
}

func (e EngineState) LastTiltfileError() error {
	return e.TiltfileState.LastBuild().Error
}

func (e *EngineState) HasDockerBuild() bool {
	for _, m := range e.Manifests() {
		for _, targ := range m.ImageTargets {
			if targ.IsDockerBuild() {
				return true
			}
		}
	}
	return false
}

func (e *EngineState) InitialBuildsCompleted() bool {
	if e.ManifestTargets == nil || len(e.ManifestTargets) == 0 {
		return false
	}

	for _, mt := range e.ManifestTargets {
		if !mt.Manifest.TriggerMode.AutoInitial() {
			continue
		}

		ms, _ := e.ManifestState(mt.Manifest.Name)
		if ms == nil || ms.LastBuild().Empty() {
			return false
		}
	}

	return true
}

// TODO(nick): This will eventually implement TargetStatus
type BuildStatus struct {
	// Stores the times of all the pending changes,
	// so we can prioritize the oldest one first.
	// This map is mutable.
	PendingFileChanges map[string]time.Time

	LastResult BuildResult

	// Stores the times that dependencies were marked dirty, so we can prioritize
	// the oldest one first.
	//
	// Long-term, we want to process all dependencies as a build graph rather than
	// a list of manifests. Specifically, we'll build one Target at a time.  Once
	// the build completes, we'll look at all the targets that depend on it, and
	// mark PendingDependencyChanges to indicate that they need a rebuild.
	//
	// Short-term, we only use this for cases where two manifests share a common
	// image. This only handles cross-manifest dependencies.
	//
	// This approach allows us to start working on the bookkeeping and
	// dependency-tracking in the short-term, without having to switch over to a
	// full dependency graph in one swoop.
	PendingDependencyChanges map[model.TargetID]time.Time
}

func newBuildStatus() *BuildStatus {
	return &BuildStatus{
		PendingFileChanges:       make(map[string]time.Time),
		PendingDependencyChanges: make(map[model.TargetID]time.Time),
	}
}

func (s BuildStatus) IsEmpty() bool {
	return len(s.PendingFileChanges) == 0 &&
		len(s.PendingDependencyChanges) == 0 &&
		s.LastResult == nil
}

func (s *BuildStatus) ClearPendingChangesBefore(startTime time.Time) {
	for file, modTime := range s.PendingFileChanges {
		if timecmp.BeforeOrEqual(modTime, startTime) {
			delete(s.PendingFileChanges, file)
		}
	}
	for file, modTime := range s.PendingDependencyChanges {
		if timecmp.BeforeOrEqual(modTime, startTime) {
			delete(s.PendingDependencyChanges, file)
		}
	}
}

type ManifestState struct {
	Name model.ManifestName

	BuildStatuses map[model.TargetID]*BuildStatus
	RuntimeState  RuntimeState

	PendingManifestChange time.Time

	// The current build
	CurrentBuild model.BuildRecord

	LastSuccessfulDeployTime time.Time

	// The last `BuildHistoryLimit` builds. The most recent build is first in the slice.
	BuildHistory []model.BuildRecord

	// The container IDs that we've run a LiveUpdate on, if any. Their contents have
	// diverged from the image they are built on. If these container don't appear on
	// the pod, we've lost that state and need to rebuild.
	LiveUpdatedContainerIDs map[container.ID]bool

	// We detected stale code and are currently doing an image build
	NeedsRebuildFromCrash bool

	// If this manifest was changed, which config files led to the most recent change in manifest definition
	ConfigFilesThatCausedChange []string

	// If the build was manually triggered, record why.
	TriggerReason model.BuildReason
}

func NewState() *EngineState {
	ret := &EngineState{}
	ret.LogStore = logstore.NewLogStore()
	ret.ManifestTargets = make(map[model.ManifestName]*ManifestTarget)
	ret.Secrets = model.SecretSet{}
	ret.DockerPruneSettings = model.DefaultDockerPruneSettings()
	ret.VersionSettings = model.VersionSettings{
		CheckUpdates: true,
	}
	ret.UpdateSettings = model.DefaultUpdateSettings()
	ret.CurrentlyBuilding = make(map[model.ManifestName]bool)
	ret.TiltfileState = &ManifestState{
		Name:          model.TiltfileManifestName,
		BuildStatuses: make(map[model.TargetID]*BuildStatus),
	}

	if ok, _ := tiltanalytics.IsAnalyticsDisabledFromEnv(); ok {
		ret.AnalyticsEnvOpt = analytics.OptOut
	}

	ret.Cmds = make(map[string]*Cmd)
	ret.FileWatches = make(map[types.NamespacedName]*v1alpha1.FileWatch)
	ret.PodLogStreams = make(map[string]*PodLogStream)
	ret.PortForwards = make(map[string]*PortForward)
	ret.KubernetesDiscoveries = make(map[types.NamespacedName]*KubernetesDiscovery)

	return ret
}

func newManifestState(m model.Manifest) *ManifestState {
	mn := m.Name
	ms := &ManifestState{
		Name:                    mn,
		BuildStatuses:           make(map[model.TargetID]*BuildStatus),
		LiveUpdatedContainerIDs: container.NewIDSet(),
	}

	if m.IsK8s() {
		ms.RuntimeState = NewK8sRuntimeState(m)
	} else if m.IsLocal() {
		ms.RuntimeState = LocalRuntimeState{}
	}

	// For historical reasons, DC state is initialized differently.

	return ms
}

func (ms *ManifestState) TargetID() model.TargetID {
	return model.TargetID{
		Type: model.TargetTypeManifest,
		Name: ms.Name.TargetName(),
	}
}

func (ms *ManifestState) BuildStatus(id model.TargetID) BuildStatus {
	result, ok := ms.BuildStatuses[id]
	if !ok {
		return BuildStatus{}
	}
	return *result
}

func (ms *ManifestState) MutableBuildStatus(id model.TargetID) *BuildStatus {
	result, ok := ms.BuildStatuses[id]
	if !ok {
		result = newBuildStatus()
		ms.BuildStatuses[id] = result
	}
	return result
}

func (ms *ManifestState) DCRuntimeState() dockercompose.State {
	ret, _ := ms.RuntimeState.(dockercompose.State)
	return ret
}

func (ms *ManifestState) IsDC() bool {
	_, ok := ms.RuntimeState.(dockercompose.State)
	return ok
}

func (ms *ManifestState) K8sRuntimeState() K8sRuntimeState {
	ret, _ := ms.RuntimeState.(K8sRuntimeState)
	return ret
}

func (ms *ManifestState) IsK8s() bool {
	_, ok := ms.RuntimeState.(K8sRuntimeState)
	return ok
}

func (ms *ManifestState) LocalRuntimeState() LocalRuntimeState {
	ret, _ := ms.RuntimeState.(LocalRuntimeState)
	return ret
}

func (ms *ManifestState) ActiveBuild() model.BuildRecord {
	return ms.CurrentBuild
}

func (ms *ManifestState) IsBuilding() bool {
	return !ms.CurrentBuild.Empty()
}

func (ms *ManifestState) LastBuild() model.BuildRecord {
	if len(ms.BuildHistory) == 0 {
		return model.BuildRecord{}
	}
	return ms.BuildHistory[0]
}

func (ms *ManifestState) AddCompletedBuild(bs model.BuildRecord) {
	ms.BuildHistory = append([]model.BuildRecord{bs}, ms.BuildHistory...)
	if len(ms.BuildHistory) > model.BuildHistoryLimit {
		ms.BuildHistory = ms.BuildHistory[:model.BuildHistoryLimit]
	}
}

func (ms *ManifestState) StartedFirstBuild() bool {
	return !ms.CurrentBuild.Empty() || len(ms.BuildHistory) > 0
}

func (ms *ManifestState) MostRecentPod() v1alpha1.Pod {
	return ms.K8sRuntimeState().MostRecentPod()
}

func (ms *ManifestState) PodWithID(pid k8s.PodID) (*v1alpha1.Pod, bool) {
	for id, pod := range ms.K8sRuntimeState().Pods {
		if id == pid {
			return pod, true
		}
	}
	return nil, false
}

func (ms *ManifestState) AddPendingFileChange(targetID model.TargetID, file string, timestamp time.Time) {
	if !ms.CurrentBuild.Empty() {
		if timestamp.Before(ms.CurrentBuild.StartTime) {
			// this file change occurred before the build started, but if the current build already knows
			// about it (from another target or rapid successive changes that weren't de-duped), it can be ignored
			for _, edit := range ms.CurrentBuild.Edits {
				if edit == file {
					return
				}
			}
		}
		// NOTE(nick): BuildController uses these timestamps to determine which files
		// to clear after a build. In particular, it:
		//
		// 1) Grabs the pending files
		// 2) Runs a live update
		// 3) Clears the pending files with timestamps before the live update started.
		//
		// Here's the race condition: suppose a file changes, but it doesn't get into
		// the EngineState until after step (2). That means step (3) will clear the file
		// even though it wasn't live-updated properly. Because as far as we can tell,
		// the file must have been in the EngineState before the build started.
		//
		// Ideally, BuildController should be do more bookkeeping to keep track of
		// which files it consumed from which FileWatches. But we're changing
		// this architecture anyway. For now, we record the time it got into
		// the EngineState, rather than the time it was originally changed.
		timestamp = time.Now()
	}

	bs := ms.MutableBuildStatus(targetID)
	bs.PendingFileChanges[file] = timestamp
}

func (ms *ManifestState) HasPendingFileChanges() bool {
	for _, status := range ms.BuildStatuses {
		if len(status.PendingFileChanges) > 0 {
			return true
		}
	}
	return false
}

func (ms *ManifestState) HasPendingDependencyChanges() bool {
	for _, status := range ms.BuildStatuses {
		if len(status.PendingDependencyChanges) > 0 {
			return true
		}
	}
	return false
}

func (mt *ManifestTarget) NextBuildReason() model.BuildReason {
	state := mt.State
	reason := state.TriggerReason
	if mt.State.HasPendingFileChanges() {
		reason = reason.With(model.BuildReasonFlagChangedFiles)
	}
	if mt.State.HasPendingDependencyChanges() {
		reason = reason.With(model.BuildReasonFlagChangedDeps)
	}
	if !mt.State.PendingManifestChange.IsZero() {
		reason = reason.With(model.BuildReasonFlagConfig)
	}
	if !mt.State.StartedFirstBuild() && mt.Manifest.TriggerMode.AutoInitial() {
		reason = reason.With(model.BuildReasonFlagInit)
	}
	if mt.State.NeedsRebuildFromCrash {
		reason = reason.With(model.BuildReasonFlagCrash)
	}
	return reason
}

// Whether changes have been made to this Manifest's synced files
// or config since the last build.
//
// Returns:
// bool: whether changes have been made
// Time: the time of the earliest change
func (ms *ManifestState) HasPendingChanges() (bool, time.Time) {
	return ms.HasPendingChangesBeforeOrEqual(time.Now())
}

// Like HasPendingChanges, but relative to a particular time.
func (ms *ManifestState) HasPendingChangesBeforeOrEqual(highWaterMark time.Time) (bool, time.Time) {
	ok := false
	earliest := highWaterMark
	t := ms.PendingManifestChange
	if !t.IsZero() && timecmp.BeforeOrEqual(t, earliest) {
		ok = true
		earliest = t
	}

	for _, status := range ms.BuildStatuses {
		for _, t := range status.PendingFileChanges {
			if !t.IsZero() && timecmp.BeforeOrEqual(t, earliest) {
				ok = true
				earliest = t
			}
		}

		for _, t := range status.PendingDependencyChanges {
			if !t.IsZero() && timecmp.BeforeOrEqual(t, earliest) {
				ok = true
				earliest = t
			}
		}
	}
	if !ok {
		return ok, time.Time{}
	}
	return ok, earliest
}

func (ms *ManifestState) UpdateStatus(triggerMode model.TriggerMode) v1alpha1.UpdateStatus {
	currentBuild := ms.CurrentBuild
	hasPendingChanges, _ := ms.HasPendingChanges()
	lastBuild := ms.LastBuild()
	lastBuildError := lastBuild.Error != nil
	hasPendingBuild := false
	if ms.TriggerReason != 0 {
		hasPendingBuild = true
	} else if triggerMode.AutoOnChange() && hasPendingChanges {
		hasPendingBuild = true
	} else if triggerMode.AutoInitial() && currentBuild.Empty() && lastBuild.Empty() {
		hasPendingBuild = true
	}

	if !currentBuild.Empty() {
		return v1alpha1.UpdateStatusInProgress
	} else if hasPendingBuild {
		return v1alpha1.UpdateStatusPending
	} else if lastBuildError {
		return v1alpha1.UpdateStatusError
	} else if !lastBuild.Empty() {
		return v1alpha1.UpdateStatusOK
	}
	return v1alpha1.UpdateStatusNone
}

var _ model.TargetStatus = &ManifestState{}

func ManifestTargetEndpoints(mt *ManifestTarget) (endpoints []model.Link) {
	if mt.Manifest.IsK8s() {
		k8sTarg := mt.Manifest.K8sTarget()
		endpoints = append(endpoints, k8sTarg.Links...)

		// If the user specified port-forwards in the Tiltfile, we
		// assume that's what they want to see in the UI (so it
		// takes precedence over any load balancer URLs
		portForwards := k8sTarg.PortForwards
		if len(portForwards) > 0 {
			for _, pf := range portForwards {
				endpoints = append(endpoints, pf.ToLink())
			}
			return endpoints
		}

		lbEndpoints := []model.Link{}
		for _, u := range mt.State.K8sRuntimeState().LBs {
			if u != nil {
				lbEndpoints = append(lbEndpoints, model.Link{URL: u})
			}
		}
		// Sort so the ordering of LB endpoints is deterministic
		// (otherwise it's not, because they live in a map)
		sort.Sort(model.ByURL(lbEndpoints))
		endpoints = append(endpoints, lbEndpoints...)
	}

	localResourceLinks := mt.Manifest.LocalTarget().Links
	if len(localResourceLinks) > 0 {
		return localResourceLinks
	}

	publishedPorts := mt.Manifest.DockerComposeTarget().PublishedPorts()
	if len(publishedPorts) > 0 {
		for _, p := range publishedPorts {
			endpoints = append(endpoints, model.MustNewLink(fmt.Sprintf("http://localhost:%d/", p), ""))
		}
	}

	endpoints = append(endpoints, mt.Manifest.DockerComposeTarget().Links...)
	return endpoints
}

func StateToView(s EngineState, mu *sync.RWMutex) view.View {
	ret := view.View{
		IsProfiling: s.IsProfiling,
	}

	ret.Resources = append(ret.Resources, tiltfileResourceView(s))

	for _, name := range s.ManifestDefinitionOrder {
		mt, ok := s.ManifestTargets[name]
		if !ok {
			continue
		}

		// Skip manifests that don't come from the tiltfile.
		if mt.Manifest.Source != model.ManifestSourceTiltfile {
			continue
		}

		ms := mt.State

		var absWatchDirs []string
		for i, p := range mt.Manifest.LocalPaths() {
			if i > 50 {
				// Bail out after 50 to avoid pathological performance issues.
				break
			}
			fi, err := os.Stat(p)

			// Treat this as a directory when there's an error.
			if err != nil || fi.IsDir() {
				absWatchDirs = append(absWatchDirs, p)
			}
		}

		var pendingBuildEdits []string
		for _, status := range ms.BuildStatuses {
			for f := range status.PendingFileChanges {
				pendingBuildEdits = append(pendingBuildEdits, f)
			}
		}

		pendingBuildEdits = ospath.FileListDisplayNames(absWatchDirs, pendingBuildEdits)

		buildHistory := append([]model.BuildRecord{}, ms.BuildHistory...)
		for i, build := range buildHistory {
			build.Edits = ospath.FileListDisplayNames(absWatchDirs, build.Edits)
			buildHistory[i] = build
		}

		currentBuild := ms.CurrentBuild
		currentBuild.Edits = ospath.FileListDisplayNames(absWatchDirs, ms.CurrentBuild.Edits)

		// Sort the strings to make the outputs deterministic.
		sort.Strings(pendingBuildEdits)

		endpoints := ManifestTargetEndpoints(mt)

		// NOTE(nick): Right now, the UX is designed to show the output exactly one
		// pod. A better UI might summarize the pods in other ways (e.g., show the
		// "most interesting" pod that's crash looping, or show logs from all pods
		// at once).
		_, pendingBuildSince := ms.HasPendingChanges()
		r := view.Resource{
			Name:               name,
			LastDeployTime:     ms.LastSuccessfulDeployTime,
			TriggerMode:        mt.Manifest.TriggerMode,
			BuildHistory:       buildHistory,
			PendingBuildEdits:  pendingBuildEdits,
			PendingBuildSince:  pendingBuildSince,
			PendingBuildReason: mt.NextBuildReason(),
			CurrentBuild:       currentBuild,
			Endpoints:          model.LinksToURLStrings(endpoints), // hud can't handle link names, just send URLs
			ResourceInfo:       resourceInfoView(mt),
		}

		ret.Resources = append(ret.Resources, r)
	}

	ret.LogReader = logstore.NewReader(mu, s.LogStore)
	ret.FatalError = s.FatalError

	return ret
}

const TiltfileManifestName = model.TiltfileManifestName

func tiltfileResourceView(s EngineState) view.Resource {
	tr := view.Resource{
		Name:         TiltfileManifestName,
		IsTiltfile:   true,
		CurrentBuild: s.TiltfileState.CurrentBuild,
		BuildHistory: s.TiltfileState.BuildHistory,
		ResourceInfo: view.TiltfileResourceInfo{},
	}
	if !s.TiltfileState.CurrentBuild.Empty() {
		tr.PendingBuildSince = s.TiltfileState.CurrentBuild.StartTime
	} else {
		tr.LastDeployTime = s.TiltfileState.LastBuild().FinishTime
	}
	return tr
}

func resourceInfoView(mt *ManifestTarget) view.ResourceInfoView {
	runStatus := v1alpha1.RuntimeStatusUnknown
	if mt.State.RuntimeState != nil {
		runStatus = mt.State.RuntimeState.RuntimeStatus()
	}

	if mt.Manifest.PodReadinessMode() == model.PodReadinessIgnore {
		return view.YAMLResourceInfo{
			K8sDisplayNames: mt.Manifest.K8sTarget().DisplayNames,
		}
	}

	switch state := mt.State.RuntimeState.(type) {
	case dockercompose.State:
		return view.NewDCResourceInfo(mt.Manifest.DockerComposeTarget().ConfigPaths,
			state.ContainerState.Status, state.ContainerID, state.SpanID, state.StartTime, runStatus)
	case K8sRuntimeState:
		pod := state.MostRecentPod()
		podID := k8s.PodID(pod.Name)
		return view.K8sResourceInfo{
			PodName:            pod.Name,
			PodCreationTime:    pod.CreatedAt.Time,
			PodUpdateStartTime: state.UpdateStartTime[podID],
			PodStatus:          pod.Status,
			PodRestarts:        int(state.VisiblePodContainerRestarts(podID)),
			SpanID:             k8sconv.SpanIDForPod(mt.Manifest.Name, podID),
			RunStatus:          runStatus,
			DisplayNames:       mt.Manifest.K8sTarget().DisplayNames,
		}
	case LocalRuntimeState:
		return view.NewLocalResourceInfo(runStatus, state.PID, state.SpanID)
	default:
		// This is silly but it was the old behavior.
		return view.K8sResourceInfo{}
	}
}

// DockerComposeConfigPath returns the path to the docker-compose yaml file of any
// docker-compose manifests on this EngineState.
// NOTE(maia): current assumption is only one d-c.yaml per run, so we take the
// path from the first d-c manifest we see.
func (s EngineState) DockerComposeConfigPath() []string {
	for _, mt := range s.ManifestTargets {
		if mt.Manifest.IsDC() {
			return mt.Manifest.DockerComposeTarget().ConfigPaths
		}
	}
	return []string{}
}
