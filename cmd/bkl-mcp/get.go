package main

import (
	"context"
	"fmt"

	"github.com/gopatchy/bkl"
)

type getArgs struct {
	Type          string `json:"type"`
	ID            string `json:"id"`
	Source        string `json:"source,omitempty"`
	ConvertToJSON *bool  `json:"convertToJSON,omitempty"`
}

type getResponse struct {
	Documentation    *bkl.DocSection `json:"documentation,omitempty"`
	Test             *bkl.DocExample `json:"test,omitempty"`
	FormatsConverted bool            `json:"formatsConverted,omitempty"`
}

func (s *Server) getHandler(ctx context.Context, args getArgs) (*getResponse, error) {
	convertToJSON := true
	if args.ConvertToJSON != nil {
		convertToJSON = *args.ConvertToJSON
	}

	response := &getResponse{}

	switch args.Type {
	case "documentation":
		for _, section := range s.sections {
			if section.ID == args.ID {
				if args.Source != "" && section.Source != args.Source {
					continue
				}

				sectionCopy := section

				if convertToJSON {
					response.FormatsConverted = sectionCopy.ConvertCodeBlocks("json-pretty")
				}

				response.Documentation = &sectionCopy
				return response, nil
			}
		}
		if args.Source != "" {
			return nil, fmt.Errorf("documentation section '%s' not found in source '%s'", args.ID, args.Source)
		}
		return nil, fmt.Errorf("documentation section '%s' not found", args.ID)

	case "test":
		test, exists := s.tests[args.ID]
		if !exists {
			return nil, fmt.Errorf("test '%s' not found", args.ID)
		}

		testCopy := *test

		if convertToJSON {
			response.FormatsConverted = testCopy.ConvertCodeBlocks("json-pretty")
		}

		response.Test = &testCopy
		return response, nil

	default:
		return nil, fmt.Errorf("invalid type '%s'. Must be 'documentation' or 'test'", args.Type)
	}
}
