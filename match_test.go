package bkl_test

import (
	"testing"

	"github.com/gopatchy/bkl"
	"github.com/stretchr/testify/require"
)

func TestMatchMap(t *testing.T) {
	t.Parallel()

	b := bkl.New()

	require.NoError(t, b.MergeFileLayers("tests/match-map/a.b.yaml"))

	blob, err := b.Output("json")
	require.NoError(t, err)
	require.Equal(t, `{"a":1,"d":4}
---
{"b":2,"c":3}
`, string(blob))
}
