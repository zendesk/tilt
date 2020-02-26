package guide

import (
	"strings"

	"github.com/windmilleng/tilt/internal/store"
)

type StateID string

type MessageFunc func(s store.EngineState) string

type AutoAdvanceFunc func(s store.EngineState) StateID

type Choice struct {
	Text      string
	NextState StateID
}

type FSMState struct {
	ID          StateID
	Message     MessageFunc
	Choices     []Choice
	AutoAdvance AutoAdvanceFunc
}

func NextState(s store.EngineState, prev StateID, choice int) (next StateID) {
	st := states[prev]
	if choice >= 0 && choice < len(st.Choices) {
		return st.Choices[choice].NextState
	}

	if st.AutoAdvance == nil {
		return ""
	}
	return st.AutoAdvance(s)
}

func enterOnboarding(s store.EngineState) bool {
	if len(s.TiltfileState.BuildHistory) < 1 {
		return false
	}

	b := s.TiltfileState.BuildHistory[0]
	if b.Error == nil {
		return false
	}
	if strings.Contains(b.Error.Error(), "No Tiltfile found at paths") {
		return true
	}

	if strings.Contains(b.Error.Error(), "No resources found.") {
		return true
	}

	return false
}

var states map[StateID]FSMState

func init() {
	states = initStates()
}

func initStates() map[StateID]FSMState {
	r := make(map[StateID]FSMState)
	for _, st := range stateList {
		r[st.ID] = st
	}
	return r
}
