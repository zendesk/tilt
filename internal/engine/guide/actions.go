package guide

import (
	"github.com/windmilleng/tilt/internal/store"
)

type UpdateGuideAction struct {
	Guide store.GuideState
}

func (UpdateGuideAction) Action() {}
