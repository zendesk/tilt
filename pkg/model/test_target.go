package model

import (
	"github.com/tilt-dev/tilt/internal/sliceutils"
)

type TestType string

var TestTypeLocal TestType = "TEST_TYPE_LOCAL"
var TestTypeCluster TestType = "TEST_TYPE_CLUSTER"

// Test Arguments:
// Name:            string
// Type:         LOCAL_TEST|TEST_CLUSTER
// Environment:     pwd|image
// Command:        string
// Tags:            []string|string
// Deps:            []file|[]service
// Timeout:

type TestTarget struct {
	Name        TargetName
	Cmd         Cmd
	Deps        []string
	Environment string   // pwd|image (default '.')
	Type        TestType // gonna be either TargetTypeLocal or TargetTypeK8s (by default, Local)
	Tags        []string
	Timeout     int

	AllowParallel bool
}

var _ TargetSpec = TestTarget{}

func NewTestTarget(name TargetName, cmd Cmd, environment string, testType TestType, tags, deps []string, timeout int) TestTarget {
	return TestTarget{
		Name:        name,
		Cmd:         cmd,
		Deps:        deps,
		Environment: environment,
		Type:        testType,
		Tags:        tags,
		Timeout:     timeout,
	}
}

func (tt TestTarget) Empty() bool {
	return tt.Cmd.Empty()
}

func (tt TestTarget) WithAllowParallel(val bool) TestTarget {
	tt.AllowParallel = val
	return tt
}

func (tt TestTarget) ID() TargetID {
	return TargetID{
		Name: tt.Name,
		// for e2e tests we're gonna want Type: TargetTypeK8s
		Type: TargetTypeLocal,
	}
}

func (tt TestTarget) DependencyIDs() []TargetID {
	return nil
}

func (tt TestTarget) Validate() error {
	return nil
}

// Implements: engine.WatchableManifest
func (tt TestTarget) Dependencies() []string {
	return sliceutils.DedupedAndSorted(tt.Deps)
}

func (tt TestTarget) LocalRepos() []LocalGitRepo {
	return nil
}

func (tt TestTarget) Dockerignores() []Dockerignore {
	return nil
}

func (tt TestTarget) IgnoredLocalDirectories() []string {
	return nil
}
