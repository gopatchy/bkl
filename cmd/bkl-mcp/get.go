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
			response.FormatsConverted = convertDocExampleCodeBlocks(&testCopy)
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
					if convertCodeBlockToJSON(item.Example.Evaluate.Inputs[j]) {
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
					if convertCodeBlockToJSON(item.Example.Intersect.Inputs[j]) {
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

func convertDocExampleCodeBlocks(test *bkl.DocExample) bool {
	converted := false

	// Convert inputs based on operation type
	switch {
	case test.Evaluate != nil:
		converted = convertDocLayerCodeBlocks(test.Evaluate.Inputs...) || converted
		converted = convertDocLayerCodeBlocks(&test.Evaluate.Result) || converted

	case test.Required != nil:
		converted = convertDocLayerCodeBlocks(test.Required.Inputs...) || converted
		converted = convertDocLayerCodeBlocks(&test.Required.Result) || converted

	case test.Diff != nil:
		converted = convertDocLayerCodeBlocks(&test.Diff.Base, &test.Diff.Target) || converted
		converted = convertDocLayerCodeBlocks(&test.Diff.Result) || converted

	case test.Intersect != nil:
		converted = convertDocLayerCodeBlocks(test.Intersect.Inputs...) || converted
		converted = convertDocLayerCodeBlocks(&test.Intersect.Result) || converted

	case test.Compare != nil:
		converted = convertDocLayerCodeBlocks(&test.Compare.Left, &test.Compare.Right) || converted
		converted = convertDocLayerCodeBlocks(&test.Compare.Result) || converted
	}

	return converted
}

func convertDocLayerCodeBlocks(layers ...*bkl.DocLayer) bool {
	converted := false

	for _, layer := range layers {
		if layer == nil {
			continue
		}

		// Determine format from filename or languages
		var ext string
		if layer.Filename != "" {
			if idx := strings.LastIndex(layer.Filename, "."); idx != -1 {
				ext = layer.Filename[idx+1:]
			}
		} else if len(layer.Languages) > 0 && len(layer.Languages[0]) > 1 {
			if format, ok := layer.Languages[0][1].(string); ok {
				ext = format
			}
		}

		if ext == "yaml" || ext == "toml" {
			formatHandler, err := format.Get(ext)
			if err != nil {
				continue
			}

			docs, err := formatHandler.UnmarshalStream([]byte(layer.Code))
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

			layer.Code = string(jsonBytes)
			layer.Content = string(jsonBytes)
			// Update languages to reflect JSON format
			if len(layer.Languages) > 0 && len(layer.Languages[0]) > 1 {
				layer.Languages[0][1] = "json"
			}
			converted = true
		}
	}

	return converted
}
