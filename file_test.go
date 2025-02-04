package bkl_test

import (
	"testing"

	"github.com/gopatchy/bkl"
	"github.com/stretchr/testify/require"
)

func TestSymlink(t *testing.T) {
	t.Parallel()

	b, err := bkl.New()
	require.NoError(t, err)

	require.NoError(t, b.MergeFileLayers("tests/symlink/c.d.yaml"))

	blob, err := b.Output("json")
	require.NoError(t, err)
	require.Equal(t, `{"a":1,"b":2,"c":3}
`, string(blob))
}

func TestParentDirective(t *testing.T) {
	t.Parallel()

	b, err := bkl.New()
	require.NoError(t, err)

	require.NoError(t, b.MergeFileLayers("tests/parent-set/a.b.yaml"))

	blob, err := b.Output("json")
	require.NoError(t, err)
	require.Equal(t, `{"a":1,"b":2}
`, string(blob))
}
