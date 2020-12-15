package triggermode

import (
	"fmt"

	"go.starlark.net/starlark"
)

const ModeN = "trigger_mode"
const ModeAutoN = "TRIGGER_MODE_AUTO"
const ModeManualN = "TRIGGER_MODE_MANUAL"

type TriggerMode int

func (t TriggerMode) String() string {
	switch t {
	case ModeAuto:
		return ModeAutoN
	case ModeManual:
		return ModeManualN
	default:
		return fmt.Sprintf("unknown trigger mode with value %d", t)
	}
}

func (t TriggerMode) Type() string {
	return "TriggerMode"
}

func (t TriggerMode) Freeze() {
	// noop
}

func (t TriggerMode) Truth() starlark.Bool {
	return starlark.MakeInt(int(t)).Truth()
}

func (t TriggerMode) Hash() (uint32, error) {
	return starlark.MakeInt(int(t)).Hash()
}

var _ starlark.Value = TriggerMode(0)

const (
	ModeUnset  TriggerMode = iota
	ModeAuto   TriggerMode = iota
	ModeManual TriggerMode = iota
)
