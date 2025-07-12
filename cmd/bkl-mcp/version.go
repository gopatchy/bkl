package main

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/gopatchy/bkl/pkg/version"
)

func (s *Server) versionHandler(ctx context.Context, args struct{}) (*debug.BuildInfo, error) {
	bi := version.GetVersion()
	if bi == nil {
		return nil, fmt.Errorf("failed to get build information")
	}
	return bi, nil
}
