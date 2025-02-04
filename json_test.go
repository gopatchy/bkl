package bkl_test

import (
	"testing"

	"github.com/gopatchy/bkl"
	"github.com/stretchr/testify/require"
)

func TestJSONInput(t *testing.T) {
	t.Parallel()

	b, err := bkl.New()
	require.NoError(t, err)

	require.NoError(t, b.MergeFileLayers("tests/json-input/a.b.json"))

	blob, err := b.Output("json-pretty")
	require.NoError(t, err)
	require.Equal(t, `{
  "a": 1,
  "b": 2
}
`, string(blob))
}
