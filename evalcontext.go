package bkl

import (
	"fmt"
	"maps"
	"os"
	"strings"
)

type EvalContext struct {
	Vars map[string]any
}

func NewEvalContext() *EvalContext {
	return &EvalContext{
		Vars: envVars(),
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

func envVars() map[string]any {
	vars := map[string]any{}

	for _, s := range os.Environ() {
		kv := strings.SplitN(s, "=", 2)
		vars[fmt.Sprintf("$env:%s", kv[0])] = kv[1]
	}

	return vars
}
