package extension

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

func TestFetchableAlreadyPresentWorks(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	f.tiltfile(`
load("ext://fetchable", "printFoo")
printFoo()
`)
	f.writeModuleLocally("fetchable", libText)

	f.assertExecOutput("foo")
}

func TestUnfetchableAlreadyPresentWorks(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	f.tiltfile(`
load("ext://unfetchable", "printFoo")
printFoo()
`)
	f.writeModuleLocally("unfetchable", libText)

	f.assertExecOutput("foo")
}

func TestFetchFetchableWorks(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	f.tiltfile(`
load("ext://fetchable", "printFoo")
printFoo()
`)

	f.assertExecOutput("foo")
}

func TestFetchUnfetchableFails(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	f.tiltfile(`
load("ext://unfetchable", "printFoo")
printFoo()
`)

	f.assertError("unfetchable can't be fetched")
}

type fixture struct {
	t   *testing.T
	skf *starkit.Fixture
	tmp *tempdir.TempDirFixture
}

func newFixture(t *testing.T) *fixture {
	tmp := tempdir.NewTempDirFixture(t)
	ext := NewExtension(
		context.Background(),
		&fakeFetcher{},
		NewLocalStore(tmp.JoinPath("project")),
	)
	skf := starkit.NewFixture(t, ext)
	skf.UseRealFS()

	return &fixture{
		t:   t,
		skf: skf,
		tmp: tmp,
	}
}

func (f *fixture) tearDown() {
	defer f.tmp.TearDown()
	defer f.skf.TearDown()
}

func (f *fixture) tiltfile(contents string) {
	f.skf.File("Tiltfile", contents)
}

func (f *fixture) assertExecOutput(expected string) {
	_, err := f.skf.ExecFile("Tiltfile")
	if err != nil {
		f.t.Fatalf("unexpected error %v", err)
	}
	if !strings.Contains(f.skf.PrintOutput(), expected) {
		f.t.Fatalf("output %q doesn't contain expected output %q", f.skf.PrintOutput(), expected)
	}
}

func (f *fixture) assertError(expected string) {
	_, err := f.skf.ExecFile("Tiltfile")
	if err == nil {
		f.t.Fatalf("expected error; got none (output %q)", f.skf.PrintOutput())
	}
	if !strings.Contains(err.Error(), expected) {
		f.t.Fatalf("error %v doens't contain expected text %q", err, expected)
	}
}

func (f *fixture) writeModuleLocally(name string, contents string) {
	f.tmp.WriteFile(filepath.Join("project", "tilt_modules", name, "Tiltfile"), contents)
}

const libText = `
def printFoo():
  print("foo")
`

type fakeFetcher struct{}

func (f *fakeFetcher) Fetch(ctx context.Context, moduleName string) (ModuleContents, error) {
	if moduleName != "fetchable" {
		return ModuleContents{}, fmt.Errorf("module %s can't be fetched because... reasons", moduleName)
	}
	return ModuleContents{
		Name:             "fetchable",
		TiltfileContents: libText,
	}, nil
}
