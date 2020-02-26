package guide

import (
	"strings"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"
)

const (
	AtStartID  StateID = "AtStart"
	WelcomeID          = "Welcome"
	Welcome2ID         = "Welcome2"
	Welcome3ID         = "Welcome3"
	K8sID              = "K8s"
	AddBuildID         = "AddBuild"
	DoneID             = "Done"
	ExitedID           = "Exited"
)

var stateList = []FSMState{
	FSMState{
		ID:          AtStartID,
		AutoAdvance: when(enterOnboarding, WelcomeID),
	},
	FSMState{
		ID:      WelcomeID,
		Message: msg("Welcome to the Tilt Guide. Let's set up a Tiltfile. To get started, click 1."),
		Choices: []Choice{{"Next", Welcome2ID}},
	},
	FSMState{
		ID:      Welcome2ID,
		Message: msg("Configure Tilt with a Tiltfile. Tilt watches this file, so you don't have to restart Tilt a bajillion times as you're getting started. Click 1."),
		Choices: []Choice{{"Next", Welcome3ID}},
	},
	FSMState{
		ID:          Welcome3ID,
		Message:     msg("Leave Tilt running and open the Tiltfile at Foo in another window. Open the Tiltfile at foo in your editor. Try adding a print statement. print('hello world'). (Waiting to see hello world in the log; click 1 to skip)"),
		Choices:     []Choice{{"Next", K8sID}},
		AutoAdvance: when(helloWorldInTiltfileLog, K8sID),
	},
	FSMState{
		ID:          K8sID,
		Message:     msg("Now add some kubernetes yaml"),
		Choices:     []Choice{{"Next", AddBuildID}},
		AutoAdvance: when(k8sManifest, AddBuildID),
	},
	FSMState{
		ID:          AddBuildID,
		Message:     msg("now add a docker build"),
		Choices:     []Choice{{"Next", DoneID}},
		AutoAdvance: when(hasBuild, DoneID),
	},
	FSMState{
		ID:      DoneID,
		Message: msg("congrats, you made it work"),
		Choices: []Choice{{"Next", ExitedID}},
	},
	FSMState{
		ID: ExitedID,
	},
}

// AutoAdvanceFuncs

func helloWorldInTiltfileLog(s store.EngineState) bool {
	return strings.Contains(s.LogStore.ManifestLog(model.TiltfileManifestName), "hello world")
}

func k8sManifest(s store.EngineState) bool {
	for _, mn := range s.ManifestDefinitionOrder {
		mt := s.ManifestTargets[mn]
		if mt == nil {
			continue
		}

		if mt.Manifest.IsK8s() {
			return true
		}
	}
	return false
}

func hasBuild(s store.EngineState) bool {
	for _, mn := range s.ManifestDefinitionOrder {
		mt := s.ManifestTargets[mn]
		if mt == nil {
			continue
		}

		if len(mt.Manifest.ImageTargets) > 0 {
			return true
		}
	}
	return false
}

// Helpers that make declaring states easier
func msg(msg string) MessageFunc {
	return func(store.EngineState) string {
		return msg
	}
}

func when(f func(store.EngineState) bool, st StateID) AutoAdvanceFunc {
	return func(s store.EngineState) StateID {
		if f(s) {
			return st
		}
		return ""
	}
}
