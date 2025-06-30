package version

import (
	"testing"
)

func TestGetVersion(t *testing.T) {
	// Just call it to make sure it doesn't crash
	_ = GetVersion()
}
