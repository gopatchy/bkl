package bkl_test

import (
	"testing"

	"github.com/gopatchy/bkl"
	"github.com/stretchr/testify/require"
)

func TestMerge(t *testing.T) {
	t.Parallel()

	b := bkl.New()

	require.NoError(t, b.MergeFileLayers("tests/merge-map/a.yaml"))

	blob, err := b.Output("json")
	require.NoError(t, err)
	require.Equal(t, `{"foo":{"bar":{"a":1}},"zig":{"a":1,"b":2}}
`, string(blob))
}

func TestReplace(t *testing.T) {
	t.Parallel()

	b := bkl.New()

	require.NoError(t, b.MergeFileLayers("tests/replace1/a.yaml"))

	blob, err := b.Output("json")
	require.NoError(t, err)
	require.Equal(t, `{"foo":{"bar":{"a":1}},"zig":{"a":1}}
`, string(blob))
}
