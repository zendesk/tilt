package tiltden

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

type HTTPClient struct {
}

func NewHTTPClient() *HTTPClient {
	return &HTTPClient{}
}

func (c *HTTPClient) Ping(ctx context.Context, token Token, team string) (Response, error) {
	var empty Response
	cl := &http.Client{
		Timeout: time.Second * 20,
	}
	d := PingSendData{
		TokenData: token.UUID.String(),
		TeamID:    team,
	}
	bs, err := json.Marshal(d)
	if err != nil {
		return empty, err
	}
	vals := url.Values{
		"data": []string{string(bs)},
	}
	resp, err := cl.PostForm("https://dbentley.ngrok.io/ping", vals)
	if err != nil {
		return empty, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return empty, err
	}

	if resp.StatusCode != http.StatusOK {
		return empty, fmt.Errorf("status %d: %s", resp.StatusCode, body)
	}

	var r Response
	if err := json.Unmarshal(body, &r); err != nil {
		return empty, err
	}

	return r, nil
}

type PingSendData struct {
	TokenData string
	TeamID    string
}
