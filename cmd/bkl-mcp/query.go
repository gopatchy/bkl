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
	Type        string `json:"type"`
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Score       int    `json:"score"`
	Source      string `json:"source,omitempty"`
}

type queryResponse struct {
	Results []queryResult `json:"results"`
}

func (s *Server) queryHandler(ctx context.Context, args queryArgs) (*queryResponse, error) {
	keywords := []string{}
	for _, kw := range strings.Split(args.Keywords, ",") {
		trimmed := strings.TrimSpace(kw)
		if trimmed != "" {
			keywords = append(keywords, strings.ToLower(trimmed))
		}
	}

	if len(keywords) == 0 {
		return nil, fmt.Errorf("no keywords provided")
	}

	allResults := []queryResult{}
	allResults = append(allResults, s.searchDocumentation(keywords)...)
	allResults = append(allResults, s.searchTests(keywords)...)

	sortResults(allResults)
	if len(allResults) > 15 {
		allResults = allResults[:15]
	}

	return &queryResponse{Results: allResults}, nil
}

func (s *Server) searchDocumentation(keywords []string) []queryResult {
	results := []queryResult{}

	for _, section := range s.sections {
		score := s.scoreDocSection(section, keywords)

		if score > 0 {
			results = append(results, queryResult{
				Type:   "documentation",
				ID:     section.ID,
				Title:  section.Title,
				Score:  score,
				Source: section.Source,
			})
		}
	}

	return results
}

func (s *Server) scoreDocSection(section bkl.DocSection, keywords []string) int {
	score := 0

	titleLower := strings.ToLower(section.Title)
	idLower := strings.ToLower(section.ID)

	score += countKeywordMatches(titleLower, keywords) * 20
	score += countKeywordMatches(idLower, keywords) * 15
	score += countKeywordMatches(section.Source, keywords) * 30

	for _, item := range section.Items {
		itemScore := scoreDocItem(item, keywords)
		score += itemScore
	}

	return score
}

func scoreDocItem(item bkl.DocItem, keywords []string) int {
	score := 0

	if item.Content != "" {
		contentLower := strings.ToLower(item.Content)
		contentMatches := countKeywordMatches(contentLower, keywords)
		if contentMatches > 0 {
			score += contentMatches * 8
		}
	}

	if item.Example != nil {
		exScore := scoreExample(item.Example, keywords)
		score += exScore
	}

	if item.Code != nil {
		codeMatches := countKeywordMatches(strings.ToLower(item.Code.Code), keywords)
		if codeMatches > 0 {
			score += codeMatches * 5
		}
	}

	if item.SideBySide != nil {
		leftMatches := countKeywordMatches(strings.ToLower(item.SideBySide.Left.Code), keywords)
		rightMatches := countKeywordMatches(strings.ToLower(item.SideBySide.Right.Code), keywords)
		score += (leftMatches + rightMatches) * 5
	}

	return score
}

func scoreExample(example *bkl.DocExample, keywords []string) int {
	score := 0

	for _, layer := range example.Layers {
		codeMatches := countKeywordMatches(strings.ToLower(layer.Code), keywords)
		labelMatches := countKeywordMatches(strings.ToLower(layer.Label), keywords)
		if codeMatches > 0 || labelMatches > 0 {
			score += (codeMatches + labelMatches) * 5
			break
		}
	}

	resultMatches := countKeywordMatches(strings.ToLower(example.Result.Code), keywords)
	score += resultMatches * 5

	return score
}

func (s *Server) searchTests(keywords []string) []queryResult {
	results := []queryResult{}

	for name, test := range s.tests {
		if strings.HasSuffix(name, ".files") {
			continue
		}

		score := s.scoreTest(name, test, keywords)

		if score > 0 {
			results = append(results, queryResult{
				Type:        "test",
				Name:        name,
				Description: test.Description,
				Score:       score,
			})
		}
	}

	return results
}

func (s *Server) scoreTest(name string, test *bkl.TestCase, keywords []string) int {
	score := 0

	nameLower := strings.ToLower(name)
	descLower := strings.ToLower(test.Description)

	score += countKeywordMatches(nameLower, keywords) * 25
	score += countKeywordMatches(descLower, keywords) * 15

	bestFileScore := 0
	for _, content := range test.Files {
		contentLower := strings.ToLower(content)
		fileMatches := countKeywordMatches(contentLower, keywords)
		if fileMatches > bestFileScore {
			bestFileScore = fileMatches
		}
	}
	score += bestFileScore * 10

	return score
}

func sortResults(results []queryResult) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
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
