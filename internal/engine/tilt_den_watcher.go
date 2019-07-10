package engine

import (
	"context"
	"time"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tiltden"
)

type TiltDenWatcher struct {
	client tiltden.Client
}

func NewTiltDenWatcher(client tiltden.Client) *TiltDenWatcher {
	return &TiltDenWatcher{client: client}
}

func (w *TiltDenWatcher) OnChange(ctx context.Context, st store.RStore) {
	state := st.RLockState()
	defer st.RUnlockState()

	tds := state.TiltDen
	if tds.TokenErr != nil {
		return
	}

	now := time.Now()

	if tds.ReqTime.After(tds.RespTime) || tds.RespTime.Add(period).After(now) {
		return
	}

	st.Dispatch(TiltDenServerRequestAction{
		Time: time.Now(),
	})

	go w.ping(ctx, st, tds.Token, "dbentley27/test1") // HARD-CODED

}

func (w *TiltDenWatcher) ping(ctx context.Context, st store.RStore, token tiltden.Token, team string) {
	resp, err := w.client.Ping(ctx, token, team)

	st.Dispatch(TiltDenServerResponseAction{
		Resp: resp,
		Err:  err,
		Time: time.Now(),
	})
}

var period = 1 * time.Second
