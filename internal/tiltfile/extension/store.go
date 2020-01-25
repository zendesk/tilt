package extension

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

const extensionDir = "tilt_modules"
const tiltfileName = "Tiltfile"

type Store interface {
	Stat(ctx context.Context, moduleName string) (string, error)
	Write(ctx context.Context, contents ModuleContents) (string, error)
}

type ModuleContents struct {
	Name             string
	TiltfileContents string
	// Should also have things like the source, versio, date fetched, etc.
}

type LocalStore struct {
	baseDir string
}

func NewLocalStore(baseDir string) *LocalStore {
	return &LocalStore{
		baseDir: filepath.Join(baseDir, extensionDir),
	}
}

func (s *LocalStore) Stat(ctx context.Context, moduleName string) (string, error) {
	tiltfilePath := filepath.Join(s.baseDir, moduleName, tiltfileName)

	_, err := os.Stat(tiltfilePath)
	if err == nil {
		return tiltfilePath, nil
	}

	if os.IsNotExist(err) {
		return "", nil
	}

	return "", err
}

func (s *LocalStore) Write(ctx context.Context, contents ModuleContents) (string, error) {
	moduleDir := filepath.Join(s.baseDir, contents.Name)
	if err := os.MkdirAll(moduleDir, os.FileMode(0700)); err != nil {
		return "", fmt.Errorf("couldn't store module %s: %v", contents.Name, err)
	}

	tiltfilePath := filepath.Join(moduleDir, tiltfileName)
	if err := ioutil.WriteFile(tiltfilePath, []byte(contents.TiltfileContents), os.FileMode(0700)); err != nil {
		return "", fmt.Errorf("couldn't store module %s: %v", contents.Name, err)
	}

	return tiltfilePath, nil
}

var _ Store = (*LocalStore)(nil)
