package tiltfile

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/pkg/model"
)

func TestTestWithDefaults(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()

	f.file("Tiltfile", `
t = test("test-foo", "echo hi")
`)

	f.load()

	foo := f.assertNextManifest("test-foo")

	require.Equal(t, "test-foo", foo.Name.String(), "manifest name")
	require.True(t, foo.IsTest(), "should be a test manifest")

	testTarg := foo.TestTarget()
	require.Equal(t, "test-foo", testTarg.Name.String(), "test target name")
	require.Equal(t, model.ToHostCmd("echo hi"), testTarg.Cmd, "test target cmd")
	require.Equal(t, f.Path(), testTarg.Environment, "default env is Tiltfile cwd")
	require.Equal(t, model.TestTypeLocal, testTarg.Type, "default type is TestTypeLocal")
}

func TestTestWithExplicit(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()

	f.file("Tiltfile", `
t = test("test-foo", "echo hi",
		deps=["a.txt", "b.txt"],
		env="gcr.io/myimg",
		type="TEST_TYPE_CLUSTER",
		tags=["beep", "boop"],
		trigger_mode=TRIGGER_MODE_MANUAL,
)
`)

	f.load()

	foo := f.assertNextManifest("test-foo")

	require.Equal(t, "test-foo", foo.Name.String(), "manifest name")
	require.True(t, foo.IsTest(), "should be a test manifest")

	testTarg := foo.TestTarget()
	require.Equal(t, "test-foo", testTarg.Name.String(), "test target name")
	require.Equal(t, model.ToHostCmd("echo hi"), testTarg.Cmd, "test target cmd")
	require.Equal(t, "gcr.io/myimg", testTarg.Environment, "test target env")
	require.Equal(t, model.TestTypeCluster, testTarg.Type, "test target type")
	require.Equal(t, []string{"beep", "boop"}, testTarg.Tags, "test target tags")
	require.Equal(t, f.JoinPaths([]string{"a.txt", "b.txt"}), testTarg.Deps, "test target deps")
	require.Equal(t, model.TriggerModeManualAfterInitial, foo.TriggerMode, "trigger mode for manifest")
}

// TODO: ACTUAL TESTS, YOU NERDS
//   - correct error when wrong type passed
//   - timeout
//   - positional vs named args
//   - allowParallel
//   - bad test type
//   - etc.
//   - how single-manifest trigger mode interactions with macro trigger mode
