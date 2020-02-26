package guide

import (
	"context"

	"github.com/windmilleng/tilt/internal/store"
)

type Controller struct {
	seqNo int64
	count int64
}

func NewController() *Controller {
	return &Controller{}
}

func (c *Controller) OnChange(ctx context.Context, st store.RStore) {
	s := st.RLockState()
	defer st.RUnlockState()

	if s.Guide.Seq < c.seqNo {
		return
	}

	changed := false
	newState := s.Guide

	if s.Guide.LastClick != "" {
		changed = true
		newState.LastClick = ""
		c.count++
	}

	if !changed {
		return
	}

	c.seqNo++
	newState.Seq = c.seqNo

	st.Dispatch(UpdateGuideAction{newState})
}
