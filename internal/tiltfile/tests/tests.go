package tests

import (
	"fmt"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/value"

	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
)

func (e Extension) NewState() interface{} {
	return []model.TestTarget{}
}

// Implements functions for dealing with k8s secret settings.
type Extension struct{}

var _ starkit.Extension = Extension{}

func NewExtension() Extension {
	return Extension{}
}

// OnStart has a useless comment on top of it
func (e Extension) OnStart(env *starkit.Environment) error {
	return env.AddBuiltin("test", e.test)
}

func (e Extension) test(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	envVal := starlark.String(".")     // default is pwd
	typ := string(model.TestTypeLocal) // this is wrong, we want a constant like TRIGGER_MODE_LOCAL
	var cmdVal, cmdBatVal starlark.Value
	deps := value.NewLocalPathListUnpacker(thread)
	var tagsVal starlark.Sequence
	var timeout int

	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
		"name", &name,
		"cmd", &cmdVal,
		"cmd_bat?", &cmdBatVal,
		"deps?", &deps,
		"env?", &envVal,
		"type?", &typ,
		"tags?", &tagsVal,
		"timeout?", &timeout,
	); err != nil {
		return nil, err
	}

	var testType model.TestType
	if typ == string(model.TestTypeLocal) {
		testType = model.TestTypeLocal
	} else if typ == string(model.TestTypeCluster) {
		testType = model.TestTypeCluster
	} else {
		return starlark.None, fmt.Errorf("bad test type: %v (b/c maia was lazy about doing this right)", typ)
	}

	cmd, err := value.ValueGroupToCmdHelper(cmdVal, cmdBatVal)
	if err != nil {
		return nil, err
	}

	var env string
	// If it's a local test, env is a path, so make sure it's an abs path
	if testType == model.TestTypeLocal {
		env, err = value.ValueToAbsPath(thread, envVal)
		if err != nil {
			return nil, err
		}
	} else {
		// otherwise it's a cluster test, env is just itself as a plain string
		env = string(envVal)
	}

	tags, err := value.SequenceToStringSlice(tagsVal)
	if err != nil {
		return nil, errors.Wrapf(err, "%s: tags", fn.Name())
	}

	// TODO: repos for paths? (see tiltfile/local_resource.go: 62)

	// TODO: set manifest-level properties like triggermode

	// TODO: can set AllowParallel from Tiltfile (or just remove it?)

	t := model.NewTestTarget(model.TargetName(name), cmd, env, testType, tags, deps.Value, timeout)

	err = starkit.SetState(thread, func(tests []model.TestTarget) []model.TestTarget {
		tests = append(tests, t)
		return tests
	})

	return starlark.None, err
}

var _ starkit.StatefulExtension = Extension{}

func MustState(model starkit.Model) []model.TestTarget {
	state, err := GetState(model)
	if err != nil {
		panic(err)
	}
	return state
}

func GetState(m starkit.Model) ([]model.TestTarget, error) {
	var state []model.TestTarget
	err := m.Load(&state)
	return state, err
}
