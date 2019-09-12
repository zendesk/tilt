package model

import ()

type LocalTarget struct {
	Name TargetName

	Cmd string

	Deps []string
}

func (t LocalTarget) ID() TargetID {
	return TargetID{
		Type: TargetTypeLocal,
		Name: t.Name,
	}
}

func (t LocalTarget) Validate() error {
	return nil
}

func (t LocalTarget) DependencyIDs() []TargetID {
	return nil
}

var _ TargetSpec = LocalTarget{}
