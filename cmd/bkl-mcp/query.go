package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/gopatchy/bkl"
)

type queryArgs struct {
	Keywords string `json:"keywords"`
}

type queryResult struct {
	Type           string   `json:"type"`
	ID             string   `json:"id,omitempty"`
	Name           string   `json:"name,omitempty"`
	Title          string   `json:"title,omitempty"`
	Description    string   `json:"description,omitempty"`
	Score          int      `json:"score"`
	URLFragment    string   `json:"url_fragment,omitempty"`
	ContentPreview string   `json:"content_preview,omitempty"`
	ExampleLabel   string   `json:"example_label,omitempty"`
	MatchingFile   string   `json:"matching_file,omitempty"`
	Source         string   `json:"source,omitempty"`
	Features       []string `json:"features,omitempty"`
}

type queryResponse struct {
	Keywords []string      `json:"keywords"`
	Results  []queryResult `json:"results"`
	Count    int           `json:"count"`
}

func (s *Server) queryHandler(ctx context.Context, args queryArgs) (*queryResponse, error) {
	keywordFields := strings.Split(args.Keywords, ",")
	keywords := []string{}
	for _, kw := range keywordFields {
		trimmed := strings.TrimSpace(kw)
		if trimmed != "" {
			keywords = append(keywords, trimmed)
		}
	}

	if len(keywords) == 0 {
		return nil, fmt.Errorf("no keywords provided")
	}

	normalizedKeywords := make([]string, len(keywords))
	for i, keyword := range keywords {
		normalizedKeywords[i] = strings.ToLower(keyword)
	}

	allResults := []queryResult{}

	for _, section := range s.sections {
		score := 0
		exampleLabel, contentPreview := "", ""

		titleLower := strings.ToLower(section.Title)
		idLower := strings.ToLower(section.ID)

		titleMatches := countKeywordMatches(titleLower, normalizedKeywords)
		idMatches := countKeywordMatches(idLower, normalizedKeywords)
		sourceMatches := countKeywordMatches(section.Source, normalizedKeywords)

		score += titleMatches * 20
		score += idMatches * 15
		score += sourceMatches * 30

		for _, item := range section.Items {
			if item.Content != "" {
				contentLower := strings.ToLower(item.Content)
				contentMatches := countKeywordMatches(contentLower, normalizedKeywords)
				if contentMatches > 0 {
					score += contentMatches * 8
					content := item.Content
					if len(content) > 200 {
						firstKeyword := findFirstKeyword(contentLower, normalizedKeywords)
						if firstKeyword != "" {
							idx := strings.Index(contentLower, firstKeyword)
							if idx >= 0 {
								start := max(0, idx-50)
								end := min(len(content), idx+150)
								content = "..." + content[start:end] + "..."
							}
						}
					}
					if contentPreview == "" {
						contentPreview = content
					}
				}
			}
			if item.Example != nil {
				for _, layer := range item.Example.Layers {
					codeMatches := countKeywordMatches(strings.ToLower(layer.Code), normalizedKeywords)
					labelMatches := countKeywordMatches(strings.ToLower(layer.Label), normalizedKeywords)
					if codeMatches > 0 || labelMatches > 0 {
						score += (codeMatches + labelMatches) * 5
						if layer.Label != "" {
							exampleLabel = layer.Label
						}
						break
					}
				}
				resultMatches := countKeywordMatches(strings.ToLower(item.Example.Result.Code), normalizedKeywords)
				if resultMatches > 0 {
					score += resultMatches * 5
				}
			}
			if item.Code != nil {
				codeMatches := countKeywordMatches(strings.ToLower(item.Code.Code), normalizedKeywords)
				if codeMatches > 0 {
					score += codeMatches * 5
					if item.Code.Label != "" {
						exampleLabel = item.Code.Label
					}
				}
			}
			if item.SideBySide != nil {
				leftMatches := countKeywordMatches(strings.ToLower(item.SideBySide.Left.Code), normalizedKeywords)
				rightMatches := countKeywordMatches(strings.ToLower(item.SideBySide.Right.Code), normalizedKeywords)
				if leftMatches > 0 || rightMatches > 0 {
					score += (leftMatches + rightMatches) * 5
				}
			}
		}

		if score > 0 {
			result := queryResult{
				Type:           "documentation",
				ID:             section.ID,
				Title:          section.Title,
				Score:          score,
				URLFragment:    "#" + section.ID,
				ContentPreview: contentPreview,
				ExampleLabel:   exampleLabel,
				Source:         section.Source,
			}
			allResults = append(allResults, result)
		}
	}

	for name, test := range s.tests {
		if strings.HasSuffix(name, ".files") {
			continue
		}

		score := 0
		matchingFile, matchingFileContent := "", ""

		nameLower := strings.ToLower(name)
		descLower := strings.ToLower(test.Description)

		nameMatches := countKeywordMatches(nameLower, normalizedKeywords)
		descMatches := countKeywordMatches(descLower, normalizedKeywords)
		bestFileMatches := 0
		for filename, content := range test.Files {
			contentLower := strings.ToLower(content)
			fileMatches := countKeywordMatches(contentLower, normalizedKeywords)
			if fileMatches > bestFileMatches {
				bestFileMatches = fileMatches
				matchingFile = filename

				if len(content) > 150 {
					firstKeyword := findFirstKeyword(contentLower, normalizedKeywords)
					if firstKeyword != "" {
						idx := strings.Index(contentLower, firstKeyword)
						if idx >= 0 {
							start := max(0, idx-30)
							end := min(len(content), idx+120)
							matchingFileContent = "..." + content[start:end] + "..."
						}
					}
				} else {
					matchingFileContent = content
				}
			}
		}

		score += nameMatches * 25
		score += descMatches * 15
		score += bestFileMatches * 10

		if score > 0 {
			result := queryResult{
				Type:           "test",
				Name:           name,
				Description:    test.Description,
				Score:          score,
				Features:       getTestFeatures(test),
				ContentPreview: matchingFileContent,
				MatchingFile:   matchingFile,
			}
			allResults = append(allResults, result)
		}
	}

	sort.Slice(allResults, func(i, j int) bool {
		if allResults[i].Score == allResults[j].Score {
			if allResults[i].Type != allResults[j].Type {
				return allResults[i].Type == "documentation"
			}
			if allResults[i].Type == "documentation" {
				return allResults[i].Title < allResults[j].Title
			}
			return allResults[i].Name < allResults[j].Name
		}
		return allResults[i].Score > allResults[j].Score
	})

	if len(allResults) > 15 {
		allResults = allResults[:15]
	}

	return &queryResponse{
		Keywords: keywords,
		Results:  allResults,
		Count:    len(allResults),
	}, nil
}

func getTestFeatures(test *bkl.TestCase) []string {
	features := []string{}

	if test.Diff {
		features = append(features, "diff")
	}
	if test.Intersect {
		features = append(features, "intersect")
	}
	if test.Required {
		features = append(features, "required")
	}
	if len(test.Errors) > 0 {
		features = append(features, "error-test")
	}
	if len(test.Files) > 1 {
		features = append(features, "multi-file")
	}

	for _, content := range test.Files {
		if strings.Contains(content, "$delete") {
			features = append(features, "$delete")
		}
		if strings.Contains(content, "$merge") {
			features = append(features, "$merge")
		}
		if strings.Contains(content, "$replace") {
			features = append(features, "$replace")
		}
		if strings.Contains(content, "$match") {
			features = append(features, "$match")
		}
		if strings.Contains(content, "$output") {
			features = append(features, "$output")
		}
		if strings.Contains(content, "$repeat") {
			features = append(features, "$repeat")
		}
		if strings.Contains(content, "$parent") {
			features = append(features, "$parent")
		}
		if strings.Contains(content, "$env:") {
			features = append(features, "$env")
		}
		if strings.Contains(content, "$\"") {
			features = append(features, "interpolation")
		}
		if strings.Contains(content, "$encode") {
			features = append(features, "$encode")
		}
		if strings.Contains(content, "$decode") {
			features = append(features, "$decode")
		}
	}

	return features
}

func countKeywordMatches(text string, keywords []string) int {
	count := 0
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			count++
		}
	}
	return count
}

func findFirstKeyword(text string, keywords []string) string {
	firstPos := len(text)
	firstKeyword := ""

	for _, keyword := range keywords {
		if pos := strings.Index(text, keyword); pos >= 0 && pos < firstPos {
			firstPos = pos
			firstKeyword = keyword
		}
	}

	return firstKeyword
}
