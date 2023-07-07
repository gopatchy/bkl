package bkl_test

import (
	"testing"

	"github.com/gopatchy/bkl"
	"github.com/stretchr/testify/require"
)

func TestMapReplace(t *testing.T) {
	t.Parallel()

	b := bkl.New()

	require.NoError(t, b.MergeFileLayers("tests/map-replace/a.b.yaml"))

	blob, err := b.Output("json")
	require.NoError(t, err)
	require.Equal(t, `{"b":2}
`, string(blob))
}

func TestListMerge(t *testing.T) {
	t.Parallel()

	b := bkl.New()

	require.NoError(t, b.MergeFileLayers("tests/list-merge/a.b.yaml"))

	blob, err := b.Output("json")
	require.NoError(t, err)
	require.Equal(t, `[1,2]
`, string(blob))
}

func TestListReplace(t *testing.T) {
	t.Parallel()

	b := bkl.New()

	require.NoError(t, b.MergeFileLayers("tests/list-replace/a.b.yaml"))

	blob, err := b.Output("json")
	require.NoError(t, err)
	require.Equal(t, `[2]
`, string(blob))
}

func TestListDelete(t *testing.T) {
	t.Parallel()

	b := bkl.New()

	require.NoError(t, b.MergeFileLayers("tests/list-delete/a.b.yaml"))

	blob, err := b.Output("json")
	require.NoError(t, err)
	require.Equal(t, `[{"x":1},{"x":3}]
`, string(blob))
}
