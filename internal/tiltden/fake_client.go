package tiltden

import (
	"context"
	"fmt"
	"time"
)

type FakeClient struct {
}

func NewFakeClient() *FakeClient {
	return &FakeClient{}
}

func (c *FakeClient) Ping(ctx context.Context, token Token, team string) (Response, error) {
	return Response{
		Message: fmt.Sprintf("hello from fake_client.go: %s", time.Now()),
	}, nil
}

var _ Client = (*FakeClient)(nil)
