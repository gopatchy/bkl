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
		score := section.Score(keywords)

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

func (s *Server) searchTests(keywords []string) []queryResult {
	results := []queryResult{}

	for name, test := range s.tests {
		if strings.HasSuffix(name, ".files") {
			continue
		}

		score := bkl.CountKeywordMatches(name, keywords) * 25
		score += test.Score(keywords)

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

func sortResults(results []queryResult) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
}
