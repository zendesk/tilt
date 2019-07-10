package tiltden

import (
	"encoding/json"
	"os"

	"github.com/google/uuid"

	"github.com/windmilleng/wmclient/pkg/dirs"
)

const tokenFile = "tilt/token.json"

type Token struct {
	UUID uuid.UUID
}

func GetOrGenerateAndSetToken() (Token, error) {
	var empty Token
	d, err := dirs.UseWindmillDir()
	if err != nil {
		return empty, err
	}

	s, err := d.ReadFile(tokenFile)
	if err == nil {
		var token Token
		if err := json.Unmarshal([]byte(s), &token); err != nil {
			return empty, err
		}
		return token, nil
	}

	if !os.IsNotExist(err) {
		return empty, err
	}

	token := Token{UUID: uuid.New()}
	bs, err := json.Marshal(token)
	if err != nil {
		return empty, err
	}

	if err := d.WriteFile(tokenFile, string(bs)); err != nil {
		return empty, err
	}

	return token, nil
}
