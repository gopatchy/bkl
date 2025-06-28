package bkl

import (
	"fmt"
	"maps"

	"github.com/gopatchy/bkl/pkg/errors"
)

type evalContext struct {
	Vars map[string]any
}

func newEvalContext(env map[string]string) *evalContext {
	vars := map[string]any{}

	for k, v := range env {
		vars[fmt.Sprintf("$env:%s", k)] = v
	}

	return &evalContext{
		Vars: vars,
	}
}

func (ec *evalContext) clone() *evalContext {
	return &evalContext{
		Vars: maps.Clone(ec.Vars),
	}
}

func (ec *evalContext) getVar(name string) (any, error) {
	v, found := ec.Vars[name]
	if found {
		return v, nil
	}

	return nil, fmt.Errorf("%s: %w", name, errors.ErrVariableNotFound)
}
