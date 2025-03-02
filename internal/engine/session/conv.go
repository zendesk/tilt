package session

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	session "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func targetsForResource(mt *store.ManifestTarget, holds buildcontrol.HoldSet) []session.Target {
	var targets []session.Target

	if bt := buildTarget(mt, holds); bt != nil {
		targets = append(targets, *bt)
	}

	if rt := runtimeTarget(mt, holds); rt != nil {
		targets = append(targets, *rt)
	}

	return targets
}

func k8sRuntimeTarget(mt *store.ManifestTarget) *session.Target {
	krs := mt.State.K8sRuntimeState()
	if mt.Manifest.PodReadinessMode() == model.PodReadinessIgnore && krs.HasEverDeployedSuccessfully && len(krs.Pods) == 0 {
		// HACK: engine assumes anything with an image will create a pod; PodReadinessIgnore is used in these
		// 	instances to avoid getting stuck in pending forever; in reality, there's no "runtime" target being
		// 	monitored by Tilt, so instead of faking it, just omit it (note: only applies AFTER first deploy so
		// 	that we can determine there are no pods, so it will appear in waiting until then, which is actually
		// 	desirable and matches behavior in K8sRuntimeState::RuntimeStatus())
		// 	see https://github.com/tilt-dev/tilt/issues/3619
		return nil
	}

	target := &session.Target{
		Name:      fmt.Sprintf("%s:runtime", mt.Manifest.Name.String()),
		Resources: []string{mt.Manifest.Name.String()},
	}

	if mt.Manifest.IsK8s() && mt.Manifest.K8sTarget().HasJob() {
		target.Type = session.TargetTypeJob
	} else {
		target.Type = session.TargetTypeServer
	}

	// a lot of this logic is duplicated from K8sRuntimeState::RuntimeStatus()
	// but ensures Job containers are handled correctly and adds additional
	// metadata
	pod := krs.MostRecentPod()
	if krs.HasEverDeployedSuccessfully && pod.Name != "" {
		switch v1.PodPhase(pod.Phase) {
		case v1.PodRunning:
			target.State.Active = &session.TargetStateActive{
				StartTime: apis.NewMicroTime(pod.CreatedAt.Time),
				Ready:     mt.Manifest.PodReadinessMode() == model.PodReadinessIgnore || store.AllPodContainersReady(pod),
			}
			return target
		case v1.PodSucceeded:
			target.State.Terminated = &session.TargetStateTerminated{
				StartTime: apis.NewMicroTime(pod.CreatedAt.Time),
			}
			return target
		case v1.PodFailed:
			podErr := strings.Join(pod.Errors, "; ")
			if podErr == "" {
				podErr = fmt.Sprintf("Pod %q failed", pod.Name)
			}
			target.State.Terminated = &session.TargetStateTerminated{
				StartTime: apis.NewMicroTime(pod.CreatedAt.Time),
				Error:     podErr,
			}
			return target
		}

		for _, ctr := range store.AllPodContainers(pod) {
			if k8sconv.ContainerStatusToRuntimeState(ctr) == v1alpha1.RuntimeStatusError {
				target.State.Terminated = &session.TargetStateTerminated{
					StartTime: apis.NewMicroTime(pod.CreatedAt.Time),
					Error: fmt.Sprintf("Pod %s in error state due to container %s: %s",
						pod.Name, ctr.Name, pod.Status),
				}
				return target
			}
		}
	}

	// for resources with auto_init=True, fake a fallback waiting state
	// for resources with auto_init=False (and the user has never triggered it),
	// 	don't populate anything to signal that the target is _intentionally_ in an "inactive" state
	if mt.Manifest.TriggerMode.AutoInitial() {
		waitReason := pod.Status
		if waitReason == "" {
			if pod.Name == "" {
				waitReason = "waiting-for-pod"
			} else {
				waitReason = "unknown"
			}
		}
		target.State.Waiting = &session.TargetStateWaiting{
			WaitReason: waitReason,
		}
	}

	return target
}

func localServeTarget(mt *store.ManifestTarget, holds buildcontrol.HoldSet) *session.Target {
	if mt.Manifest.LocalTarget().ServeCmd.Empty() {
		// there is no serve_cmd, so don't return a runtime target at all
		// (there will still be a build target from the update cmd)
		return nil
	}

	target := &session.Target{
		Name:      fmt.Sprintf("%s:serve", mt.Manifest.Name.String()),
		Resources: []string{mt.Manifest.Name.String()},
		Type:      session.TargetTypeServer,
	}

	lrs := mt.State.LocalRuntimeState()
	if runtimeErr := lrs.RuntimeStatusError(); runtimeErr != nil {
		target.State.Terminated = &session.TargetStateTerminated{
			StartTime:  apis.NewMicroTime(lrs.StartTime),
			FinishTime: apis.NewMicroTime(lrs.FinishTime),
			Error:      errToString(runtimeErr),
		}
	} else if lrs.PID != 0 {
		target.State.Active = &session.TargetStateActive{
			StartTime: apis.NewMicroTime(lrs.StartTime),
			Ready:     lrs.Ready,
		}
	} else if mt.Manifest.TriggerMode.AutoInitial() || mt.State.StartedFirstBuild() {
		// default to waiting unless this resource has auto_init=False and has never
		// had a build triggered for other reasons (e.g. trigger_mode=TRIGGER_MODE_AUTO and
		// a relevant file change or being manually invoked via UI)
		// the latter case ensures there's no race condition between a build being
		// triggered and the local process actually being launched
		//
		// otherwise, Terminated/Active/Waiting will all be nil, which indicates that
		// the target is currently inactive
		target.State.Waiting = waitingFromHolds(mt.Manifest.Name, holds)
	}

	return target
}

// genericRuntimeTarget creates a target from the RuntimeState interface without any domain-specific considerations.
//
// This is both used for target types that don't require specialized logic (Docker Compose) as well as a fallback for
// any new types that don't have deeper support here.
func genericRuntimeTarget(mt *store.ManifestTarget, holds buildcontrol.HoldSet) *session.Target {
	target := &session.Target{
		Name:      fmt.Sprintf("%s:runtime", mt.Manifest.Name.String()),
		Resources: []string{mt.Manifest.Name.String()},
		Type:      session.TargetTypeServer,
	}

	// HACK: RuntimeState is not populated until engine starts builds in some cases; to avoid weird race conditions,
	// 	it defaults to pending assuming the resource isn't _actually_ disabled on startup via auto_init=False
	var runtimeStatus v1alpha1.RuntimeStatus
	if mt.State.RuntimeState != nil {
		runtimeStatus = mt.State.RuntimeState.RuntimeStatus()
	} else if mt.Manifest.TriggerMode.AutoInitial() {
		runtimeStatus = v1alpha1.RuntimeStatusPending
	}

	switch runtimeStatus {
	case v1alpha1.RuntimeStatusPending:
		target.State.Waiting = waitingFromHolds(mt.Manifest.Name, holds)
	case v1alpha1.RuntimeStatusOK:
		target.State.Active = &session.TargetStateActive{
			StartTime: apis.NewMicroTime(mt.State.LastSuccessfulDeployTime),
			// generic resources have no readiness concept so they're just ready by default
			// (this also applies to Docker Compose, since we don't support its health checks)
			Ready: true,
		}
	case v1alpha1.RuntimeStatusError:
		errMsg := errToString(mt.State.RuntimeState.RuntimeStatusError())
		if errMsg == "" {
			errMsg = "Server target %q failed"
		}
		target.State.Terminated = &session.TargetStateTerminated{
			Error: errMsg,
		}
	}

	return target
}

func runtimeTarget(mt *store.ManifestTarget, holds buildcontrol.HoldSet) *session.Target {
	if mt.Manifest.IsK8s() {
		return k8sRuntimeTarget(mt)
	} else if mt.Manifest.IsLocal() {
		return localServeTarget(mt, holds)
	} else {
		return genericRuntimeTarget(mt, holds)
	}
}

// buildTarget creates a "build" (or update) target for the resource.
//
// Currently, the engine aggregates many different targets into a single build record, and that's reflected here.
// Ideally, as the internals change, more granularity will provided and this might actually return a slice of targets
// rather than a single target. For example, a K8s resource might have an image build step and then a deployment (i.e.
// kubectl apply) step - currently, both of these will be aggregated together, which can make it harder to diagnose
// where something is stuck or slow.
func buildTarget(mt *store.ManifestTarget, holds buildcontrol.HoldSet) *session.Target {
	if mt.Manifest.IsLocal() && mt.Manifest.LocalTarget().UpdateCmd.Empty() {
		return nil
	}

	res := &session.Target{
		Name:      fmt.Sprintf("%s:update", mt.Manifest.Name.String()),
		Resources: []string{mt.Manifest.Name.String()},
		Type:      session.TargetTypeJob,
	}

	isPending := mt.NextBuildReason() != model.BuildReasonNone
	if isPending {
		res.State.Waiting = waitingFromHolds(mt.Manifest.Name, holds)
	} else if !mt.State.CurrentBuild.Empty() {
		res.State.Active = &session.TargetStateActive{
			StartTime: apis.NewMicroTime(mt.State.CurrentBuild.StartTime),
		}
	} else if len(mt.State.BuildHistory) != 0 {
		lastBuild := mt.State.LastBuild()
		res.State.Terminated = &session.TargetStateTerminated{
			StartTime:  apis.NewMicroTime(lastBuild.StartTime),
			FinishTime: apis.NewMicroTime(lastBuild.FinishTime),
			Error:      errToString(lastBuild.Error),
		}
	}

	return res
}

func waitingFromHolds(mn model.ManifestName, holds buildcontrol.HoldSet) *session.TargetStateWaiting {
	// in the API, the reason is not _why_ the target "exists", but rather an explanation for why it's not yet
	// active and is in a pending state (e.g. waitingFromHolds for dependencies)
	waitReason := "unknown"
	if hold, ok := holds[mn]; ok && hold != store.HoldNone {
		waitReason = string(hold)
	}
	return &session.TargetStateWaiting{
		WaitReason: waitReason,
	}
}

// tiltfileTarget creates a session.Target object from the engine state for the current Tiltfile.
//
// This is slightly different from generic resource handling because there is no ManifestTarget in the engine
// for the Tiltfile (just ManifestState) and config file changes are stored stop level on state, but conceptually
// it does similar things.
func tiltfileTarget(state store.EngineState) session.Target {
	target := session.Target{
		Name:      "tiltfile:update",
		Resources: []string{model.TiltfileManifestName.String()},
		Type:      session.TargetTypeJob,
	}

	// Tiltfile is special in engine state and doesn't have a target, just state, so
	// this logic is largely duplicated from the generic resource build logic
	if !state.TiltfileState.CurrentBuild.Empty() {
		target.State.Active = &session.TargetStateActive{
			StartTime: apis.NewMicroTime(state.TiltfileState.CurrentBuild.StartTime),
		}
	} else if hasPendingChanges, _ := state.TiltfileState.HasPendingChanges(); hasPendingChanges {
		target.State.Waiting = &session.TargetStateWaiting{
			WaitReason: "config-changed",
		}
	} else if len(state.TiltfileState.BuildHistory) != 0 {
		lastBuild := state.TiltfileState.LastBuild()
		target.State.Terminated = &session.TargetStateTerminated{
			StartTime:  apis.NewMicroTime(lastBuild.StartTime),
			FinishTime: apis.NewMicroTime(lastBuild.FinishTime),
			Error:      errToString(lastBuild.Error),
		}
	} else {
		// given the current engine behavior, this doesn't actually occur because
		// the first build happens as part of initialization
		target.State.Waiting = &session.TargetStateWaiting{
			WaitReason: "initial-build",
		}
	}

	return target
}
