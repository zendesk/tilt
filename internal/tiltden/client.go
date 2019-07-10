package tiltden

import (
	"context"
)

type Response struct {
	Message string
}

type Client interface {
	Ping(ctx context.Context, token Token, team string) (Response, error)
}
