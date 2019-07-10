package model

import (
	"time"

	"github.com/windmilleng/tilt/internal/tiltden"
)

type TiltDenStatus struct {
	// set at startup and never changed
	Token    tiltden.Token
	TokenErr error
	Team     string

	// request that's running
	ReqTime time.Time

	// result from TiltDen Server
	Resp     tiltden.Response
	RespErr  error
	RespTime time.Time
}
