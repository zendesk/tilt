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

func (tt TestTarget) ToManifest() Manifest {
	return Manifest{
		Name: ManifestName(tt.Name),

		// 🤖 we'll make a new struct TestTarget that satisfies interface DeployTarget
		// (see model.LocalTarget) that will eventually hold info like file deps, command
		// to run the test, etc.
		// Will also have a manifest.IsTest() func (see `manifest.IsLocal`) to tell us if it's a test or not.
		// Eventually will teach a BuildAndDeployer how to deal with Test targets, that's how
		// they'll actually get run (see `LocalTargetBuildAndDeployer`)
		// The advantage of using a manifest is that we already know how to watch a manifest for
		// file changes, collect logs, send through the (increasingly poorly named) build and deploy
		// pipeline when we detect a change, etc.
		DeployTarget: tt,

		// TODO: can set these
		TriggerMode:          0,
		ResourceDependencies: nil,
		Source:               ManifestSourceTiltfile,
	}
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
