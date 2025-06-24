package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/gopatchy/bkl"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var (
	tests    map[string]*bkl.MCPTestCase
	sections []bkl.MCPDocSection
)

func loadData() error {
	var err error

	// Load tests from bkl package
	tests, err = bkl.GetMCPTests()
	if err != nil {
		return fmt.Errorf("failed to load tests: %v", err)
	}

	// Load documentation sections from bkl package
	sections, err = bkl.GetMCPDocSections()
	if err != nil {
		return fmt.Errorf("failed to load documentation sections: %v", err)
	}

	return nil
}

func main() {
	// Load embedded data
	if err := loadData(); err != nil {
		log.Fatalf("Failed to load data: %v", err)
	}

	// Create a new MCP server
	mcpServer := server.NewMCPServer(
		"bkl-mcp",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// Register tools
	queryTool := mcp.NewTool("query",
		mcp.WithDescription("Query bkl documentation and test examples by keywords"),
		mcp.WithString("keywords",
			mcp.Required(),
			mcp.Description("Keywords to search for (comma-separated) in documentation sections and test examples"),
		),
	)
	mcpServer.AddTool(queryTool, queryHandler)

	getTool := mcp.NewTool("get",
		mcp.WithDescription("Get full content of a documentation section or test"),
		mcp.WithString("type",
			mcp.Required(),
			mcp.Description("Type of content: 'documentation' or 'test'"),
		),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("ID of documentation section or name of test"),
		),
	)
	mcpServer.AddTool(getTool, getHandler)

	// Start the stdio transport
	if err := server.ServeStdio(mcpServer); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func queryHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	keywordsStr, err := request.RequireString("keywords")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Split keywords by comma and normalize
	keywordFields := strings.Split(keywordsStr, ",")
	var keywords []string
	for _, kw := range keywordFields {
		trimmed := strings.TrimSpace(kw)
		if trimmed != "" {
			keywords = append(keywords, trimmed)
		}
	}

	if len(keywords) == 0 {
		return mcp.NewToolResultError("No keywords provided"), nil
	}

	// Normalize keywords to lowercase
	normalizedKeywords := make([]string, len(keywords))
	for i, keyword := range keywords {
		normalizedKeywords[i] = strings.ToLower(keyword)
	}

	var allResults []map[string]interface{}

	// Search documentation sections
	for _, section := range sections {
		score := 0
		details := map[string]interface{}{}

		// Check section title and ID
		titleLower := strings.ToLower(section.Title)
		idLower := strings.ToLower(section.ID)

		titleMatches := countKeywordMatches(titleLower, normalizedKeywords)
		idMatches := countKeywordMatches(idLower, normalizedKeywords)

		score += titleMatches * 20
		score += idMatches * 15

		// Check content items
		matchingContent := []string{}
		for _, item := range section.Items {
			if item.Type == "text" {
				contentLower := strings.ToLower(item.Content)
				contentMatches := countKeywordMatches(contentLower, normalizedKeywords)
				if contentMatches > 0 {
					score += contentMatches * 8
					// Extract relevant snippet for first matching keyword
					content := item.Content
					if len(content) > 200 {
						// Find first keyword position and extract context
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
					matchingContent = append(matchingContent, content)
				}
			}
			if item.Type == "example" {
				// Check example code and labels
				if item.Example.Type == "grid" {
					for _, row := range item.Example.Rows {
						for _, gridItem := range row.Items {
							codeMatches := countKeywordMatches(strings.ToLower(gridItem.Code), normalizedKeywords)
							labelMatches := countKeywordMatches(strings.ToLower(gridItem.Label), normalizedKeywords)
							if codeMatches > 0 || labelMatches > 0 {
								score += (codeMatches + labelMatches) * 5
								if gridItem.Label != "" {
									details["example_label"] = gridItem.Label
								}
								break
							}
						}
					}
				} else {
					codeMatches := countKeywordMatches(strings.ToLower(item.Example.Code), normalizedKeywords)
					if codeMatches > 0 {
						score += codeMatches * 5
						if item.Example.Label != "" {
							details["example_label"] = item.Example.Label
						}
					}
				}
			}
		}

		if score > 0 {
			result := map[string]interface{}{
				"type":         "documentation",
				"id":           section.ID,
				"title":        section.Title,
				"score":        score,
				"url_fragment": "#" + section.ID,
			}
			if len(matchingContent) > 0 {
				result["content_preview"] = matchingContent[0]
			}
			for k, v := range details {
				result[k] = v
			}
			allResults = append(allResults, result)
		}
	}

	// Search tests
	for name, test := range tests {
		if strings.HasSuffix(name, ".files") {
			continue
		}

		score := 0
		details := map[string]interface{}{}

		nameLower := strings.ToLower(name)
		descLower := strings.ToLower(test.Description)

		nameMatches := countKeywordMatches(nameLower, normalizedKeywords)
		descMatches := countKeywordMatches(descLower, normalizedKeywords)

		// Check for keywords in file contents
		var matchingFileContent string
		var bestFileMatches int
		for filename, content := range test.Files {
			contentLower := strings.ToLower(content)
			fileMatches := countKeywordMatches(contentLower, normalizedKeywords)
			if fileMatches > bestFileMatches {
				bestFileMatches = fileMatches
				details["matching_file"] = filename
				// Extract snippet around first matching keyword
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
			result := map[string]interface{}{
				"type":        "test",
				"name":        name,
				"description": test.Description,
				"score":       score,
				"features":    getTestFeatures(test),
			}
			if matchingFileContent != "" {
				result["content_preview"] = matchingFileContent
			}
			for k, v := range details {
				result[k] = v
			}
			allResults = append(allResults, result)
		}
	}

	// Sort all results by score descending, then by type (docs first)
	sort.Slice(allResults, func(i, j int) bool {
		scoreI := allResults[i]["score"].(int)
		scoreJ := allResults[j]["score"].(int)
		if scoreI == scoreJ {
			// If scores are equal, prioritize documentation
			typeI := allResults[i]["type"].(string)
			typeJ := allResults[j]["type"].(string)
			if typeI != typeJ {
				return typeI == "documentation"
			}
			// Then sort by name/title
			if typeI == "documentation" {
				return allResults[i]["title"].(string) < allResults[j]["title"].(string)
			}
			return allResults[i]["name"].(string) < allResults[j]["name"].(string)
		}
		return scoreI > scoreJ
	})

	// Limit to top 15 results
	if len(allResults) > 15 {
		allResults = allResults[:15]
	}

	response := map[string]interface{}{
		"keywords": keywords,
		"results":  allResults,
		"count":    len(allResults),
	}

	resultJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(string(resultJSON)), nil
}

func getHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	contentType, err := request.RequireString("type")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	id, err := request.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	switch contentType {
	case "documentation":
		for _, section := range sections {
			if section.ID == id {
				sectionJSON, err := json.MarshalIndent(section, "", "  ")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				return mcp.NewToolResultText(string(sectionJSON)), nil
			}
		}
		return mcp.NewToolResultText(fmt.Sprintf("Documentation section '%s' not found", id)), nil

	case "test":
		test, exists := tests[id]
		if !exists {
			return mcp.NewToolResultText(fmt.Sprintf("Test '%s' not found", id)), nil
		}

		testJSON, err := json.MarshalIndent(test, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(testJSON)), nil

	default:
		return mcp.NewToolResultError(fmt.Sprintf("Invalid type '%s'. Must be 'documentation' or 'test'", contentType)), nil
	}
}

func getTestFeatures(test *bkl.MCPTestCase) []string {
	var features []string

	if test.Diff {
		features = append(features, "diff")
	}
	if test.Intersect {
		features = append(features, "intersect")
	}
	if test.Required {
		features = append(features, "required")
	}
	if test.Error != "" {
		features = append(features, "error-test")
	}
	if len(test.Files) > 1 {
		features = append(features, "multi-file")
	}

	// Check for special directives in file contents
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// countKeywordMatches counts how many keywords from the list are found in the text
func countKeywordMatches(text string, keywords []string) int {
	count := 0
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			count++
		}
	}
	return count
}

// findFirstKeyword returns the first keyword found in the text
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
