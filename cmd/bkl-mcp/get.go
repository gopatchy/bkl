package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/gopatchy/bkl"
	"github.com/gopatchy/bkl/internal/format"
)

type getArgs struct {
	Type          string `json:"type"`
	ID            string `json:"id"`
	Source        string `json:"source,omitempty"`
	ConvertToJSON *bool  `json:"convertToJSON,omitempty"`
}

type getResponse struct {
	Documentation    *bkl.DocSection `json:"documentation,omitempty"`
	Test             *bkl.TestCase   `json:"test,omitempty"`
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
					response.FormatsConverted = convertDocSectionCodeBlocks(&sectionCopy)
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
			response.FormatsConverted = convertTestCaseCodeBlocks(&testCopy)
		}

		response.Test = &testCopy
		return response, nil

	default:
		return nil, fmt.Errorf("invalid type '%s'. Must be 'documentation' or 'test'", args.Type)
	}
}

func convertCodeBlockToJSON(layer *bkl.DocLayer) bool {
	if len(layer.Languages) != 1 || len(layer.Languages[0]) != 1 {
		return false
	}

	lang, ok := layer.Languages[0][0].(string)
	if !ok {
		return false
	}

	if lang != "yaml" && lang != "toml" {
		return false
	}

	formatHandler, err := format.Get(lang)
	if err != nil {
		return false
	}

	docs, err := formatHandler.UnmarshalStream([]byte(layer.Code))
	if err != nil {
		return false
	}

	jsonHandler, err := format.Get("json-pretty")
	if err != nil {
		return false
	}

	jsonBytes, err := jsonHandler.MarshalStream(docs)
	if err != nil {
		return false
	}

	layer.Code = string(jsonBytes)
	layer.Languages[0][0] = "json"
	return true
}

func convertDocSectionCodeBlocks(section *bkl.DocSection) bool {
	converted := false

	for i := range section.Items {
		item := &section.Items[i]

		if item.Example != nil {
			switch {
			case item.Example.Evaluate != nil:
				for j := range item.Example.Evaluate.Inputs {
					if convertCodeBlockToJSON(&item.Example.Evaluate.Inputs[j]) {
						converted = true
					}
				}
				if convertCodeBlockToJSON(&item.Example.Evaluate.Result) {
					converted = true
				}

			case item.Example.Diff != nil:
				if convertCodeBlockToJSON(&item.Example.Diff.Base) {
					converted = true
				}
				if convertCodeBlockToJSON(&item.Example.Diff.Target) {
					converted = true
				}
				if convertCodeBlockToJSON(&item.Example.Diff.Result) {
					converted = true
				}

			case item.Example.Intersect != nil:
				for j := range item.Example.Intersect.Inputs {
					if convertCodeBlockToJSON(&item.Example.Intersect.Inputs[j]) {
						converted = true
					}
				}
				if convertCodeBlockToJSON(&item.Example.Intersect.Result) {
					converted = true
				}

			case item.Example.Convert != nil:
				if convertCodeBlockToJSON(&item.Example.Convert.From) {
					converted = true
				}
				if convertCodeBlockToJSON(&item.Example.Convert.To) {
					converted = true
				}

			case item.Example.Fixit != nil:
				if item.Example.Fixit.Original.Code != "" {
					if convertCodeBlockToJSON(&item.Example.Fixit.Original) {
						converted = true
					}
				}
				if convertCodeBlockToJSON(&item.Example.Fixit.Bad) {
					converted = true
				}
				if convertCodeBlockToJSON(&item.Example.Fixit.Good) {
					converted = true
				}

			case item.Example.Compare != nil:
				if convertCodeBlockToJSON(&item.Example.Compare.Left) {
					converted = true
				}
				if convertCodeBlockToJSON(&item.Example.Compare.Right) {
					converted = true
				}
				if convertCodeBlockToJSON(&item.Example.Compare.Result) {
					converted = true
				}
			}
		}

		if item.Code != nil {
			if convertCodeBlockToJSON(item.Code) {
				converted = true
			}
		}

		if item.SideBySide != nil {
			if convertCodeBlockToJSON(&item.SideBySide.Left) {
				converted = true
			}
			if convertCodeBlockToJSON(&item.SideBySide.Right) {
				converted = true
			}
		}
	}

	return converted
}

func convertTestCaseCodeBlocks(test *bkl.TestCase) bool {
	converted := false

	for filename, content := range test.Files {
		ext := strings.TrimPrefix(filename[strings.LastIndex(filename, "."):], ".")
		if ext == "yaml" || ext == "toml" {
			formatHandler, err := format.Get(ext)
			if err != nil {
				continue
			}

			docs, err := formatHandler.UnmarshalStream([]byte(content))
			if err != nil {
				continue
			}

			jsonHandler, err := format.Get("json-pretty")
			if err != nil {
				continue
			}

			jsonBytes, err := jsonHandler.MarshalStream(docs)
			if err != nil {
				continue
			}

			test.Files[filename] = string(jsonBytes)
			converted = true
		}
	}

	if test.Format == "yaml" || test.Format == "toml" {
		formatHandler, err := format.Get(test.Format)
		if err == nil {
			docs, err := formatHandler.UnmarshalStream([]byte(test.Expected))
			if err == nil {
				jsonHandler, err := format.Get("json-pretty")
				if err == nil {
					jsonBytes, err := jsonHandler.MarshalStream(docs)
					if err == nil {
						test.Expected = string(jsonBytes)
						test.Format = "json"
						converted = true
					}
				}
			}
		}
	}

	return converted
}
