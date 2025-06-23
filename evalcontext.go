package bkl

import (
	"fmt"
	"maps"
)

type EvalContext struct {
	Vars map[string]any
}

func NewEvalContext(env map[string]string) *EvalContext {
	vars := map[string]any{}

	for k, v := range env {
		vars[fmt.Sprintf("$env:%s", k)] = v
	}

	return &EvalContext{
		Vars: vars,
	}
}

func (ec *EvalContext) Clone() *EvalContext {
	return &EvalContext{
		Vars: maps.Clone(ec.Vars),
	}
}

func (ec *EvalContext) GetVar(name string) (any, error) {
	v, found := ec.Vars[name]
	if found {
		return v, nil
	}

	return nil, fmt.Errorf("%s: %w", name, ErrVariableNotFound)
}
