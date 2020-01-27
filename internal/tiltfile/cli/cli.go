package cli

import (
	"fmt"

	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

type Extension struct{}

func NewExtension() Extension {
	return Extension{}
}

type Resources map[string]string

func (Extension) NewState() interface{} {
	return make(Resources)
}

func (Extension) OnStart(e *starkit.Environment) error {
	return e.AddBuiltin("cli_resource", cliResource)
}

func cliResource(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	var cmd string
	err := starkit.UnpackArgs(t, fn.Name(), args, kwargs, "name", &name, "cmd", &cmd)
	if err != nil {
		return nil, err
	}

	err = starkit.SetState(t, func(in Resources) (Resources, error) {
		if existing, ok := in[name]; ok {
			return nil, fmt.Errorf("cli_resource named %v already declared with cmd %v", name, existing)
		}
		in[name] = cmd
		return in, nil
	})

	if err != nil {
		return nil, err
	}

	return starlark.None, nil
}

func GetState(m starkit.Model) (r Resources, err error) {
	err = m.Load(&r)
	return
}
