package file

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type matchExpression struct {
	filename string
	match    any
}

func parseMatchExpression(expr string) (*matchExpression, error) {
	parts := strings.SplitN(expr, ":", 2)
	if len(parts) == 1 {
		return &matchExpression{filename: expr}, nil
	}

	filename := parts[0]
	matchStr := parts[1]

	if filename == "" {
		return nil, fmt.Errorf("empty filename in match expression")
	}

	matchStr = strings.TrimSpace(matchStr)
	if matchStr == "" {
		return nil, fmt.Errorf("empty match expression")
	}

	var match any
	if err := yaml.Unmarshal([]byte(matchStr), &match); err != nil {
		return nil, fmt.Errorf("invalid match expression: %w", err)
	}

	return &matchExpression{
		filename: filename,
		match:    match,
	}, nil
}
