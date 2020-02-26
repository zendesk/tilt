package guide

import (
	"context"
	"strconv"

	"github.com/windmilleng/tilt/internal/store"
)

type Controller struct {
	seqNo   int64
	current StateID
}

func NewController() *Controller {
	return &Controller{
		current: AtStartID,
	}
}

func (c *Controller) OnChange(ctx context.Context, st store.RStore) {
	s := st.RLockState()
	defer st.RUnlockState()

	if s.Guide.Seq < c.seqNo {
		return
	}

	// make LastClick an Int
	choice, err := strconv.Atoi(s.Guide.LastClick)
	if err != nil {
		choice = -1
	}

	n := NextState(s, c.current, choice)

	if n == "" {
		return
	}

	c.current = n

	c.seqNo++

	var r store.GuideState
	r.Seq = c.seqNo
	if states[n].Message != nil {
		r.Message = states[n].Message(s)
	}
	r.LastClick = ""

	st.Dispatch(UpdateGuideAction{r})
}
