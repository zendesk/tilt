package engine

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/tilt-dev/wmclient/pkg/analytics"

	tiltanalytics "github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"
	"github.com/tilt-dev/tilt/internal/engine/configs"
	"github.com/tilt-dev/tilt/internal/engine/dcwatch"
	"github.com/tilt-dev/tilt/internal/engine/fswatch"
	"github.com/tilt-dev/tilt/internal/engine/k8swatch"
	"github.com/tilt-dev/tilt/internal/engine/local"
	"github.com/tilt-dev/tilt/internal/engine/portforward"
	"github.com/tilt-dev/tilt/internal/engine/runtimelog"
	"github.com/tilt-dev/tilt/internal/engine/session"
	"github.com/tilt-dev/tilt/internal/hud"
	"github.com/tilt-dev/tilt/internal/hud/prompt"
	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/timecmp"
	"github.com/tilt-dev/tilt/internal/token"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

// TODO(nick): maybe this should be called 'BuildEngine' or something?
// Upper seems like a poor and undescriptive name.
type Upper struct {
	store *store.Store
}

type ServiceWatcherMaker func(context.Context, *store.Store) error
type PodWatcherMaker func(context.Context, *store.Store) error

func NewUpper(ctx context.Context, st *store.Store, subs []store.Subscriber) (Upper, error) {
	// There's not really a good reason to add all the subscribers
	// in NewUpper(), but it's as good a place as any.
	for _, sub := range subs {
		err := st.AddSubscriber(ctx, sub)
		if err != nil {
			return Upper{}, err
		}
	}

	return Upper{
		store: st,
	}, nil
}

func (u Upper) Dispatch(action store.Action) {
	u.store.Dispatch(action)
}

func (u Upper) Start(
	ctx context.Context,
	args []string,
	b model.TiltBuild,
	engineMode store.EngineMode,
	fileName string,
	initTerminalMode store.TerminalMode,
	analyticsUserOpt analytics.Opt,
	token token.Token,
	cloudAddress string,
) error {

	startTime := time.Now()

	absTfPath, err := filepath.Abs(fileName)
	if err != nil {
		return err
	}

	configFiles := []string{absTfPath}

	return u.Init(ctx, InitAction{
		EngineMode:       engineMode,
		TiltfilePath:     absTfPath,
		ConfigFiles:      configFiles,
		UserArgs:         args,
		TiltBuild:        b,
		StartTime:        startTime,
		AnalyticsUserOpt: analyticsUserOpt,
		Token:            token,
		CloudAddress:     cloudAddress,
		TerminalMode:     initTerminalMode,
	})
}

func (u Upper) Init(ctx context.Context, action InitAction) error {
	u.store.Dispatch(action)
	return u.store.Loop(ctx)
}

func upperReducerFn(ctx context.Context, state *store.EngineState, action store.Action) {
	// Allow exitAction and dumpEngineStateAction even if there's a fatal error
	if exitAction, isExitAction := action.(hud.ExitAction); isExitAction {
		handleHudExitAction(state, exitAction)
		return
	}
	if _, isDumpEngineStateAction := action.(hud.DumpEngineStateAction); isDumpEngineStateAction {
		handleDumpEngineStateAction(ctx, state)
		return
	}

	if state.FatalError != nil {
		return
	}

	switch action := action.(type) {
	case InitAction:
		handleInitAction(ctx, state, action)
	case store.ErrorAction:
		state.FatalError = action.Error
	case hud.ExitAction:
		handleHudExitAction(state, action)
	case fswatch.FileWatchCreateAction:
		fswatch.HandleFileWatchCreateEvent(ctx, state, action)
	case fswatch.FileWatchUpdateAction:
		fswatch.HandleFileWatchUpdateEvent(ctx, state, action)
	case fswatch.FileWatchUpdateStatusAction:
		fswatch.HandleFileWatchUpdateStatusEvent(ctx, state, action)
	case fswatch.FileWatchDeleteAction:
		fswatch.HandleFileWatchDeleteEvent(ctx, state, action)
	case k8swatch.KubernetesDiscoveryCreateAction:
		k8swatch.HandleKubernetesDiscoveryCreateAction(ctx, state, action)
	case k8swatch.KubernetesDiscoveryUpdateAction:
		k8swatch.HandleKubernetesDiscoveryUpdateAction(ctx, state, action)
	case k8swatch.KubernetesDiscoveryUpdateStatusAction:
		k8swatch.HandleKubernetesDiscoveryUpdateStatusAction(ctx, state, action)
	case k8swatch.KubernetesDiscoveryDeleteAction:
		k8swatch.HandleKubernetesDiscoveryDeleteAction(ctx, state, action)
	case k8swatch.ServiceChangeAction:
		handleServiceEvent(ctx, state, action)
	case store.K8sEventAction:
		handleK8sEvent(ctx, state, action)
	case buildcontrol.BuildCompleteAction:
		handleBuildCompleted(ctx, state, action)
	case buildcontrol.BuildStartedAction:
		handleBuildStarted(ctx, state, action)
	case configs.ConfigsReloadStartedAction:
		handleConfigsReloadStarted(ctx, state, action)
	case configs.ConfigsReloadedAction:
		handleConfigsReloaded(ctx, state, action)
	case dcwatch.EventAction:
		handleDockerComposeEvent(ctx, state, action)
	case server.AppendToTriggerQueueAction:
		state.AppendToTriggerQueue(action.Name, action.Reason)
	case hud.StartProfilingAction:
		handleStartProfilingAction(state)
	case hud.StopProfilingAction:
		handleStopProfilingAction(state)
	case hud.DumpEngineStateAction:
		handleDumpEngineStateAction(ctx, state)
	case store.AnalyticsUserOptAction:
		handleAnalyticsUserOptAction(state, action)
	case store.AnalyticsNudgeSurfacedAction:
		handleAnalyticsNudgeSurfacedAction(ctx, state)
	case store.TiltCloudStatusReceivedAction:
		handleTiltCloudStatusReceivedAction(state, action)
	case store.UserStartedTiltCloudRegistrationAction:
		handleUserStartedTiltCloudRegistrationAction(state)
	case store.PanicAction:
		handlePanicAction(state, action)
	case server.SetTiltfileArgsAction:
		handleSetTiltfileArgsAction(state, action)
	case store.LogAction:
		handleLogAction(state, action)
	case session.SessionUpdateStatusAction:
		session.HandleSessionUpdateStatusAction(state, action)
	case prompt.SwitchTerminalModeAction:
		handleSwitchTerminalModeAction(state, action)
	case server.OverrideTriggerModeAction:
		handleOverrideTriggerModeAction(ctx, state, action)
	case local.CmdCreateAction:
		local.HandleCmdCreateAction(state, action)
	case local.CmdUpdateStatusAction:
		local.HandleCmdUpdateStatusAction(state, action)
	case local.CmdDeleteAction:
		local.HandleCmdDeleteAction(state, action)
	case runtimelog.PodLogStreamCreateAction:
		runtimelog.HandlePodLogStreamCreateAction(state, action)
	case runtimelog.PodLogStreamDeleteAction:
		runtimelog.HandlePodLogStreamDeleteAction(state, action)
	case portforward.PortForwardUpsertAction:
		portforward.HandlePortForwardUpsertAction(state, action)
	case portforward.PortForwardDeleteAction:
		portforward.HandlePortForwardDeleteAction(state, action)
	default:
		state.FatalError = fmt.Errorf("unrecognized action: %T", action)
	}
}

var UpperReducer = store.Reducer(upperReducerFn)

func handleBuildStarted(ctx context.Context, state *store.EngineState, action buildcontrol.BuildStartedAction) {
	state.StartedBuildCount++

	mn := action.ManifestName
	manifest, ok := state.Manifest(mn)
	if !ok {
		return
	}

	ms, ok := state.ManifestState(mn)
	if !ok {
		return
	}

	bs := model.BuildRecord{
		Edits:     append([]string{}, action.FilesChanged...),
		StartTime: action.StartTime,
		Reason:    action.Reason,
		SpanID:    action.SpanID,
	}
	ms.ConfigFilesThatCausedChange = []string{}
	ms.CurrentBuild = bs

	if ms.IsK8s() {
		krs := ms.K8sRuntimeState()
		for podID := range krs.Pods {
			krs.UpdateStartTime[podID] = action.StartTime
		}
		// remove stale pods
		for podID := range krs.UpdateStartTime {
			if _, ok := krs.Pods[podID]; !ok {
				delete(krs.UpdateStartTime, podID)
			}
		}
	} else if manifest.IsDC() {
		// Attach the SpanID and initialize the runtime state if we haven't yet.
		state, _ := ms.RuntimeState.(dockercompose.State)
		state = state.WithSpanID(runtimelog.SpanIDForDCService(mn))
		ms.RuntimeState = state
	}

	// If this is a full build, we know all the containers will get replaced,
	// so just reset them now.
	//
	// NOTE(nick): Currently, this addresses an issue where the full build deletes
	// the deployment, which then starts killing pods, which we interpret as a
	// crash. A better way to resolve this problem would be to watch for deletions
	// directly. But it's still semantically correct to record that we intended to
	// delete the containers.
	if action.FullBuildTriggered {
		// Reset all the container ids
		ms.LiveUpdatedContainerIDs = container.NewIDSet()
	}

	state.CurrentlyBuilding[mn] = true
	state.RemoveFromTriggerQueue(mn)
}

// When a Manifest build finishes, update the BuildStatus for all applicable
// targets in the engine state.
func handleBuildResults(engineState *store.EngineState,
	mt *store.ManifestTarget, br model.BuildRecord, results store.BuildResultSet) {
	isBuildSuccess := br.Error == nil

	ms := mt.State
	mn := mt.Manifest.Name
	for id, result := range results {
		ms.MutableBuildStatus(id).LastResult = result
	}

	// Remove pending file changes that were consumed by this build.
	for _, status := range ms.BuildStatuses {
		status.ClearPendingChangesBefore(br.StartTime)
	}

	if isBuildSuccess {
		ms.LastSuccessfulDeployTime = br.FinishTime
	}

	// Update build statuses for duplicated image targets in other manifests.
	// This ensures that those images targets aren't redundantly rebuilt.
	for _, currentMT := range engineState.TargetsBesides(mn) {
		// We only want to update image targets for Manifests that are already queued
		// for rebuild and not currently building. This has two benefits:
		//
		// 1) If there's a bug in Tilt's image caching (e.g.,
		//    https://github.com/tilt-dev/tilt/pull/3542), this prevents infinite
		//    builds.
		//
		// 2) If the current manifest build was kicked off by a trigger, we don't
		//    want to queue manifests with the same image.
		if currentMT.NextBuildReason() == model.BuildReasonNone ||
			engineState.IsCurrentlyBuilding(currentMT.Manifest.Name) {
			continue
		}

		currentMS := currentMT.State
		idSet := currentMT.Manifest.TargetIDSet()
		updatedIDSet := make(map[model.TargetID]bool)

		for id, result := range results {
			_, ok := idSet[id]
			if !ok {
				continue
			}

			// We can only reuse image update, not live-updates or other kinds of
			// deploys.
			_, isImageResult := result.(store.ImageBuildResult)
			if !isImageResult {
				continue
			}

			currentStatus := currentMS.MutableBuildStatus(id)
			currentStatus.LastResult = result
			currentStatus.ClearPendingChangesBefore(br.StartTime)
			updatedIDSet[id] = true
		}

		if len(updatedIDSet) == 0 {
			continue
		}

		// Suppose we built manifestA, which contains imageA depending on imageCommon.
		//
		// We also have manifestB, which contains imageB depending on imageCommon.
		//
		// We need to mark imageB as dirty, because it was not built in the manifestA
		// build but its dependency was built.
		//
		// Note that this logic also applies to deploy targets depending on image
		// targets. If we built manifestA, which contains imageX and deploy target
		// k8sA, and manifestB contains imageX and deploy target k8sB, we need to mark
		// target k8sB as dirty so that Tilt actually deploys the changes to imageX.
		rDepsMap := currentMT.Manifest.ReverseDependencyIDs()
		for updatedID := range updatedIDSet {

			// Go through each target depending on an image we just built.
			for _, rDepID := range rDepsMap[updatedID] {

				// If that target was also built, it's up-to-date.
				if updatedIDSet[rDepID] {
					continue
				}

				// Otherwise, we need to mark it for rebuild to pick up the new image.
				currentMS.MutableBuildStatus(rDepID).PendingDependencyChanges[updatedID] = br.StartTime
			}
		}
	}
}

func handleBuildCompleted(ctx context.Context, engineState *store.EngineState, cb buildcontrol.BuildCompleteAction) {
	defer func() {
		delete(engineState.CurrentlyBuilding, cb.ManifestName)
	}()

	engineState.CompletedBuildCount++

	mt, ok := engineState.ManifestTargets[cb.ManifestName]
	if !ok {
		return
	}

	err := cb.Error
	if err != nil {
		s := fmt.Sprintf("Build Failed: %v", err)
		handleLogAction(engineState, store.NewLogAction(mt.Manifest.Name, cb.SpanID, logger.ErrorLvl, nil, []byte(s)))
	}

	ms := mt.State
	bs := ms.CurrentBuild
	bs.Error = err
	bs.FinishTime = cb.FinishTime
	bs.BuildTypes = cb.Result.BuildTypes()
	if bs.SpanID != "" {
		bs.WarningCount = len(engineState.LogStore.Warnings(bs.SpanID))
	}

	ms.AddCompletedBuild(bs)

	ms.CurrentBuild = model.BuildRecord{}
	ms.NeedsRebuildFromCrash = false

	handleBuildResults(engineState, mt, bs, cb.Result)

	if !ms.PendingManifestChange.IsZero() &&
		timecmp.BeforeOrEqual(ms.PendingManifestChange, bs.StartTime) {
		ms.PendingManifestChange = time.Time{}
	}

	if err != nil {
		if buildcontrol.IsFatalError(err) {
			engineState.FatalError = err
			return
		}
	} else {
		krs := ms.K8sRuntimeState()
		for podID, pod := range krs.Pods {
			// Reset the baseline, so that we don't show restarts
			// from before any live-updates
			krs.BaselineRestarts[podID] = store.AllPodContainerRestarts(*pod)
		}
	}

	// Track the container ids that have been live-updated whether the
	// build succeeds or fails.
	liveUpdateContainerIDs := cb.Result.LiveUpdatedContainerIDs()
	if len(liveUpdateContainerIDs) == 0 {
		// Assume this was an image build, and reset all the container ids
		ms.LiveUpdatedContainerIDs = container.NewIDSet()
	} else {
		for _, cID := range liveUpdateContainerIDs {
			ms.LiveUpdatedContainerIDs[cID] = true
		}

		krs := ms.K8sRuntimeState()
		bestPod := krs.MostRecentPod()
		if timecmp.AfterOrEqual(bestPod.CreatedAt, bs.StartTime) ||
			timecmp.Equal(krs.UpdateStartTime[k8s.PodID(bestPod.Name)], bs.StartTime) {
			k8swatch.CheckForContainerCrash(engineState, mt)
		}
	}

	manifest := mt.Manifest
	if manifest.IsK8s() {
		state := ms.K8sRuntimeState()
		deployedEntities := cb.Result.DeployedEntities()
		if len(deployedEntities) > 0 {
			state.DeployedEntities = deployedEntities
		}

		deployedPodTemplateSpecHashSet := cb.Result.DeployedPodTemplateSpecHashes()
		if len(deployedPodTemplateSpecHashSet) > 0 {
			state.DeployedPodTemplateSpecHashSet = deployedPodTemplateSpecHashSet
		}

		if err == nil {
			state.HasEverDeployedSuccessfully = true
		}

		ms.RuntimeState = state
	}

	if mt.Manifest.IsDC() {
		state, _ := ms.RuntimeState.(dockercompose.State)

		result := cb.Result[mt.Manifest.DockerComposeTarget().ID()]
		dcResult, _ := result.(store.DockerComposeBuildResult)
		cid := dcResult.DockerComposeContainerID
		if cid != "" {
			state = state.WithContainerID(cid)
		}

		cState := dcResult.ContainerState
		if cState != nil {
			state = state.WithContainerState(*cState)

			if docker.HasStarted(*cState) {
				if state.StartTime.IsZero() {
					state = state.WithStartTime(cb.FinishTime)
				}
				if state.LastReadyTime.IsZero() {
					// NB: this will differ from StartTime once we support DC health checks
					state = state.WithLastReadyTime(cb.FinishTime)
				}
			}
		}

		ms.RuntimeState = state
	}

	if mt.Manifest.IsLocal() {
		lrs := ms.LocalRuntimeState()
		if err == nil {
			lt := mt.Manifest.LocalTarget()
			if lt.ReadinessProbe == nil {
				// only update the succeeded time if there's no readiness probe
				lrs.LastReadyOrSucceededTime = time.Now()
			}
			if lt.ServeCmd.Empty() {
				// local resources without a serve command are jobs that run and
				// terminate; so there's no real runtime status
				lrs.Status = v1alpha1.RuntimeStatusNotApplicable
			}
		}
		ms.RuntimeState = lrs
	}
}

func handleStopProfilingAction(state *store.EngineState) {
	state.IsProfiling = false
}

func handleStartProfilingAction(state *store.EngineState) {
	state.IsProfiling = true
}

func handleConfigsReloadStarted(
	ctx context.Context,
	state *store.EngineState,
	event configs.ConfigsReloadStartedAction,
) {
	status := model.BuildRecord{
		StartTime: event.StartTime,
		Reason:    event.Reason,
		Edits:     event.FilesChanged,
		SpanID:    event.SpanID,
	}

	state.TiltfileState.CurrentBuild = status
	state.RemoveFromTriggerQueue(model.TiltfileManifestName)
	state.StartedTiltfileLoadCount++
}

func handleConfigsReloaded(
	ctx context.Context,
	state *store.EngineState,
	event configs.ConfigsReloadedAction,
) {

	manifests := event.Manifests
	manifestNames := map[model.ManifestName]bool{}
	for _, m := range manifests {
		manifestNames[m.Name] = true
	}

	b := state.TiltfileState.CurrentBuild

	// Remove pending file changes that were consumed by this build.
	for _, status := range state.TiltfileState.BuildStatuses {
		status.ClearPendingChangesBefore(b.StartTime)
	}

	// Track the new secrets and go back to scrub them.
	newSecrets := model.SecretSet{}
	for k, v := range event.Secrets {
		_, exists := state.Secrets[k]
		if !exists {
			newSecrets[k] = v
		}
	}

	// Add all secrets, even if we failed.
	state.Secrets.AddAll(event.Secrets)

	// Retroactively scrub secrets
	state.LogStore.ScrubSecretsStartingAt(newSecrets, event.CheckpointAtExecStart)

	// Add tiltignore if it exists, even if execution failed.
	if !event.Tiltignore.Empty() || event.Err == nil {
		state.Tiltignore = event.Tiltignore
	}

	if !event.WatchSettings.Empty() || event.Err == nil {
		state.WatchSettings = event.WatchSettings
	}

	// Add team id if it exists, even if execution failed.
	if event.TeamID != "" || event.Err == nil {
		state.TeamID = event.TeamID
	}

	// Only set the metrics settings from the tiltfile if the mode
	// hasn't been configured from elsewhere.
	if state.MetricsServing.Mode == model.MetricsDefault {
		// Add metrics if it exists, even if execution failed.
		if event.MetricsSettings.Enabled || event.Err == nil {
			state.MetricsSettings = event.MetricsSettings
		}
	}

	// if the ConfigsReloadedAction came from a unit test, there might not be a current build
	if !b.Empty() {
		b.FinishTime = event.FinishTime
		b.Error = event.Err

		if b.SpanID != "" {
			b.WarningCount = len(state.LogStore.Warnings(b.SpanID))
		}

		state.TiltfileState.AddCompletedBuild(b)
	}
	state.TiltfileState.CurrentBuild = model.BuildRecord{}
	if event.Err != nil {
		// When the Tiltfile had an error, we want to differentiate between two cases:
		//
		// 1) You're running `tilt up` for the first time, and a local() command
		// exited with status code 1.  Partial results (like enabling features)
		// would be helpful.
		//
		// 2) You're running 'tilt up' in the happy state. You edit the Tiltfile,
		// and introduce a syntax error.  You don't want partial results to wipe out
		// your "good" state.

		// Watch any new config files in the partial state.
		state.ConfigFiles = sliceutils.AppendWithoutDupes(state.ConfigFiles, event.ConfigFiles...)

		// Enable any new features in the partial state.
		if len(state.Features) == 0 {
			state.Features = event.Features
		} else {
			for feature, val := range event.Features {
				if val {
					state.Features[feature] = val
				}
			}
		}
		return
	}

	state.DockerPruneSettings = event.DockerPruneSettings

	newDefOrder := make([]model.ManifestName, len(manifests))
	for i, m := range manifests {
		mt, ok := state.ManifestTargets[m.ManifestName()]
		if !ok {
			mt = store.NewManifestTarget(m)
		}

		newDefOrder[i] = m.ManifestName()

		configFilesThatChanged := state.TiltfileState.LastBuild().Edits
		old := mt.Manifest
		mt.Manifest = m

		if model.ChangesInvalidateBuild(old, m) {
			// Manifest has changed such that the current build is invalid;
			// ensure we do an image build so that we apply the changes
			ms := mt.State
			ms.BuildStatuses = make(map[model.TargetID]*store.BuildStatus)
			ms.PendingManifestChange = event.FinishTime
			ms.ConfigFilesThatCausedChange = configFilesThatChanged
		}
		state.UpsertManifestTarget(mt)
	}

	// Go through all the existing manifest targets:
	// 1) If they were from the Tiltfile, but were removed from the latest
	//    Tiltfile execution, delete them.
	// 2) If they were not from the Tiltfile, preserve them.
	for _, mt := range state.Targets() {
		m := mt.Manifest
		if m.Source == model.ManifestSourceTiltfile {
			if !manifestNames[m.Name] {
				delete(state.ManifestTargets, m.Name)
			}
			continue
		}

		newDefOrder = append(newDefOrder, m.Name)
	}

	// TODO(maia): update ConfigsManifest with new ConfigFiles/update watches

	state.ManifestDefinitionOrder = newDefOrder
	state.ConfigFiles = event.ConfigFiles

	state.Features = event.Features
	state.TelemetrySettings = event.TelemetrySettings
	state.VersionSettings = event.VersionSettings
	state.AnalyticsTiltfileOpt = event.AnalyticsTiltfileOpt

	state.UpdateSettings = event.UpdateSettings
}

func handleLogAction(state *store.EngineState, action store.LogAction) {
	manifest, ok := state.Manifest(action.ManifestName())
	if ok && manifest.Source == model.ManifestSourceMetrics {
		return
	}
	state.LogStore.Append(action, state.Secrets)
}

func handleSwitchTerminalModeAction(state *store.EngineState, action prompt.SwitchTerminalModeAction) {
	state.TerminalMode = action.Mode
}

func handleServiceEvent(ctx context.Context, state *store.EngineState, action k8swatch.ServiceChangeAction) {
	service := action.Service
	ms, ok := state.ManifestState(action.ManifestName)
	if !ok {
		return
	}

	runtime := ms.K8sRuntimeState()
	runtime.LBs[k8s.ServiceName(service.Name)] = action.URL
}

func handleK8sEvent(ctx context.Context, state *store.EngineState, action store.K8sEventAction) {
	// TODO(nick): I think we whould so something more intelligent here, where we
	// have special treatment for different types of events, e.g.:
	//
	// - Attach Image Pulling/Pulled events to the pod state, and display how much
	//   time elapsed between them.
	// - Display Node unready events as part of a health indicator, and display how
	//   long it takes them to resolve.
	handleLogAction(state, action.ToLogAction(action.ManifestName))
}

func handleDumpEngineStateAction(ctx context.Context, engineState *store.EngineState) {
	f, err := ioutil.TempFile("", "tilt-engine-state-*.txt")
	if err != nil {
		logger.Get(ctx).Infof("error creating temp file to write engine state: %v", err)
		return
	}

	logger.Get(ctx).Infof("dumped tilt engine state to %q", f.Name())
	spew.Fdump(f, engineState)

	err = f.Close()
	if err != nil {
		logger.Get(ctx).Infof("error closing engine state temp file: %v", err)
		return
	}
}

func handleInitAction(ctx context.Context, engineState *store.EngineState, action InitAction) {
	engineState.TiltBuildInfo = action.TiltBuild
	engineState.TiltStartTime = action.StartTime
	engineState.TiltfilePath = action.TiltfilePath
	engineState.ConfigFiles = action.ConfigFiles
	engineState.UserConfigState.Args = action.UserArgs
	engineState.AnalyticsUserOpt = action.AnalyticsUserOpt
	engineState.EngineMode = action.EngineMode
	engineState.CloudAddress = action.CloudAddress
	engineState.Token = action.Token
	engineState.TerminalMode = action.TerminalMode
}

func handleHudExitAction(state *store.EngineState, action hud.ExitAction) {
	if action.Err != nil {
		state.FatalError = action.Err
	} else {
		state.UserExited = true
	}
}

func handlePanicAction(state *store.EngineState, action store.PanicAction) {
	state.PanicExited = action.Err
}

func handleSetTiltfileArgsAction(state *store.EngineState, action server.SetTiltfileArgsAction) {
	state.UserConfigState = state.UserConfigState.WithArgs(action.Args)
}

func handleDockerComposeEvent(ctx context.Context, engineState *store.EngineState, action dcwatch.EventAction) {
	evt := action.Event
	mn := model.ManifestName(evt.Service)
	ms, ok := engineState.ManifestState(mn)
	if !ok {
		// No corresponding manifest, nothing to do
		return
	}

	state, _ := ms.RuntimeState.(dockercompose.State)

	state = state.WithContainerID(container.ID(evt.ID)).
		WithSpanID(runtimelog.SpanIDForDCService(mn)).
		WithContainerState(action.ContainerState)

	if evt.IsStartupEvent() {
		state = state.WithStartTime(action.Time)
		// NB: this will differ from StartTime once we support DC health checks
		state = state.WithLastReadyTime(action.Time)
	}

	ms.RuntimeState = state
}

func handleAnalyticsUserOptAction(state *store.EngineState, action store.AnalyticsUserOptAction) {
	state.AnalyticsUserOpt = action.Opt
}

// The first time we hear that the analytics nudge was surfaced, record a metric.
// We double check !state.AnalyticsNudgeSurfaced -- i.e. that the state doesn't
// yet know that we've surfaced the nudge -- to ensure that we only record this
// metric once (since it's an anonymous metric, we can't slice it by e.g. # unique
// users, so the numbers need to be as accurate as possible).
func handleAnalyticsNudgeSurfacedAction(ctx context.Context, state *store.EngineState) {
	if !state.AnalyticsNudgeSurfaced {
		tiltanalytics.Get(ctx).Incr("analytics.nudge.surfaced", nil)
		state.AnalyticsNudgeSurfaced = true
	}
}

func handleTiltCloudStatusReceivedAction(state *store.EngineState, action store.TiltCloudStatusReceivedAction) {
	if action.IsPostRegistrationLookup {
		state.CloudStatus.WaitingForStatusPostRegistration = false
	}
	if !action.Found {
		state.CloudStatus.TokenKnownUnregistered = true
		state.CloudStatus.Username = ""
	} else {
		state.CloudStatus.TokenKnownUnregistered = false
		state.CloudStatus.Username = action.Username
		state.CloudStatus.TeamName = action.TeamName
	}

	state.SuggestedTiltVersion = action.SuggestedTiltVersion
}

func handleUserStartedTiltCloudRegistrationAction(state *store.EngineState) {
	state.CloudStatus.WaitingForStatusPostRegistration = true
}

func handleOverrideTriggerModeAction(ctx context.Context, state *store.EngineState,
	action server.OverrideTriggerModeAction) {
	// TODO(maia): in this implementation, overrides do NOT persist across Tiltfile loads
	//   (i.e. the next Tiltfile load will wipe out the override we just put in place).
	//   If we want to keep this functionality, the next step is to store the set of overrides
	//   on the engine state, and whenever we load the manifest from the Tiltfile, apply
	//   any necessary overrides.

	// We validate trigger mode when we receive a request, so this should never happen
	if !model.ValidTriggerMode(action.TriggerMode) {
		logger.Get(ctx).Errorf("INTERNAL ERROR overriding trigger mode: invalid trigger mode %d", action.TriggerMode)
		return
	}

	for _, mName := range action.ManifestNames {
		mt, ok := state.ManifestTargets[mName]
		if !ok {
			// We validate manifest names when we receive a request, so this should never happen
			logger.Get(ctx).Errorf("INTERNAL ERROR overriding trigger mode: no such manifest %q", mName)
			return
		}
		mt.Manifest.TriggerMode = action.TriggerMode
	}
}
