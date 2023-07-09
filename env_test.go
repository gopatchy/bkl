package bkl_test

import (
	"testing"

	"github.com/gopatchy/bkl"
	"github.com/stretchr/testify/require"
)

func TestEnv(t *testing.T) { //nolint:paralleltest
	t.Setenv("FOO", "xyz")

	b := bkl.New()

	require.NoError(t, b.MergeFileLayers("tests/env-map-value/a.yaml"))

	blob, err := b.Output("json")
	require.NoError(t, err)
	require.Equal(t, `{"a":"xyz"}
`, string(blob))
}
