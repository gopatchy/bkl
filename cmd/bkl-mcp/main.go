package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"sort"
	"strings"
	"testing/fstest"

	"github.com/gopatchy/bkl"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var (
	tests    map[string]*bkl.TestCase
	sections []bkl.DocSection
)

func loadData() error {
	var err error

	tests, err = bkl.GetTests()
	if err != nil {
		return fmt.Errorf("failed to load tests: %v", err)
	}

	sections, err = bkl.GetDocSections()
	if err != nil {
		return fmt.Errorf("failed to load documentation sections: %v", err)
	}

	return nil
}

func main() {
	if err := loadData(); err != nil {
		log.Fatalf("Failed to load data: %v", err)
	}

	mcpServer := server.NewMCPServer(
		"bkl-mcp",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	formatParam := mcp.WithString("format",
		mcp.Description("Output format (yaml, json, toml) - will auto-detect if not specified"),
	)
	fileSystemParam := mcp.WithObject("fileSystem",
		mcp.Required(),
		mcp.Description("Map of filename to file content for the operation"),
	)

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

	evaluateTool := mcp.NewTool("evaluate",
		mcp.WithDescription("Evaluate bkl files with given environment and return results"),
		mcp.WithString("files",
			mcp.Required(),
			mcp.Description("Comma-separated list of files to evaluate (relative paths)"),
		),
		formatParam,
		mcp.WithObject("environment",
			mcp.Description("Environment variables as key-value pairs"),
		),
		fileSystemParam,
	)
	mcpServer.AddTool(evaluateTool, evaluateHandler)

	diffTool := mcp.NewTool("diff",
		mcp.WithDescription("Generate the minimal intermediate layer needed to create the target output from the base layer"),
		mcp.WithString("baseFile",
			mcp.Required(),
			mcp.Description("Base file path"),
		),
		mcp.WithString("targetFile",
			mcp.Required(),
			mcp.Description("Target file path"),
		),
		formatParam,
		fileSystemParam,
	)
	mcpServer.AddTool(diffTool, diffHandler)

	intersectTool := mcp.NewTool("intersect",
		mcp.WithDescription("Generate the maximal base layer that the specified targets have in common"),
		mcp.WithString("files",
			mcp.Required(),
			mcp.Description("Comma-separated list of files to intersect (requires at least 2 files)"),
		),
		formatParam,
		fileSystemParam,
	)
	mcpServer.AddTool(intersectTool, intersectHandler)

	requiredTool := mcp.NewTool("required",
		mcp.WithDescription("Generate a document containing just the required fields and their ancestors from the lower layer"),
		mcp.WithString("file",
			mcp.Required(),
			mcp.Description("File path to extract required fields from"),
		),
		formatParam,
		fileSystemParam,
	)
	mcpServer.AddTool(requiredTool, requiredHandler)

	if err := server.ServeStdio(mcpServer); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func queryHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	keywordsStr, err := request.RequireString("keywords")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

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

	normalizedKeywords := make([]string, len(keywords))
	for i, keyword := range keywords {
		normalizedKeywords[i] = strings.ToLower(keyword)
	}

	var allResults []map[string]interface{}

	for _, section := range sections {
		score := 0
		details := map[string]interface{}{}

		titleLower := strings.ToLower(section.Title)
		idLower := strings.ToLower(section.ID)

		titleMatches := countKeywordMatches(titleLower, normalizedKeywords)
		idMatches := countKeywordMatches(idLower, normalizedKeywords)

		score += titleMatches * 20
		score += idMatches * 15

		matchingContent := []string{}
		for _, item := range section.Items {
			if item.Content != "" {
				contentLower := strings.ToLower(item.Content)
				contentMatches := countKeywordMatches(contentLower, normalizedKeywords)
				if contentMatches > 0 {
					score += contentMatches * 8
					// Extract relevant snippet for first matching keyword
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
					matchingContent = append(matchingContent, content)
				}
			}
			if item.Example != nil {
				// Check example layers
				for _, layer := range item.Example.Layers {
					codeMatches := countKeywordMatches(strings.ToLower(layer.Code), normalizedKeywords)
					labelMatches := countKeywordMatches(strings.ToLower(layer.Label), normalizedKeywords)
					if codeMatches > 0 || labelMatches > 0 {
						score += (codeMatches + labelMatches) * 5
						if layer.Label != "" {
							details["example_label"] = layer.Label
						}
						break
					}
				}
				// Also check result layer
				resultMatches := countKeywordMatches(strings.ToLower(item.Example.Result.Code), normalizedKeywords)
				if resultMatches > 0 {
					score += resultMatches * 5
				}
			}
			// Check simple code blocks
			if item.Code != nil {
				codeMatches := countKeywordMatches(strings.ToLower(item.Code.Code), normalizedKeywords)
				if codeMatches > 0 {
					score += codeMatches * 5
					if item.Code.Label != "" {
						details["example_label"] = item.Code.Label
					}
				}
			}
			// Check side-by-side code blocks
			if item.SideBySide != nil {
				leftMatches := countKeywordMatches(strings.ToLower(item.SideBySide.Left.Code), normalizedKeywords)
				rightMatches := countKeywordMatches(strings.ToLower(item.SideBySide.Right.Code), normalizedKeywords)
				if leftMatches > 0 || rightMatches > 0 {
					score += (leftMatches + rightMatches) * 5
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

func getTestFeatures(test *bkl.TestCase) []string {
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

func parseFileSystem(args map[string]interface{}) (map[string]string, error) {
	fileSystemRaw := args["fileSystem"]
	if fileSystemRaw == nil {
		return nil, fmt.Errorf("fileSystem parameter is required")
	}

	fileSystemMap, ok := fileSystemRaw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("fileSystem must be an object")
	}

	fileSystem := make(map[string]string)
	for k, v := range fileSystemMap {
		if str, ok := v.(string); ok {
			fileSystem[k] = str
		} else {
			return nil, fmt.Errorf("fileSystem[%s] must be a string, got %T", k, v)
		}
	}

	return fileSystem, nil
}

func parseOptionalString(args map[string]interface{}, key string, defaultValue string) string {
	if val := args[key]; val != nil {
		if str, ok := val.(string); ok && str != "" {
			return str
		}
	}
	return defaultValue
}

func createTestFS(fileSystem map[string]string) (fs.FS, error) {
	fsys := fstest.MapFS{}
	for filename, content := range fileSystem {
		fsys[filename] = &fstest.MapFile{
			Data: []byte(content),
		}
	}

	return fsys, nil
}

func parseEnvironment(args map[string]interface{}) (map[string]string, error) {
	if envRaw := args["environment"]; envRaw != nil {
		if envMap, ok := envRaw.(map[string]interface{}); ok {
			env := make(map[string]string)
			for k, v := range envMap {
				if str, ok := v.(string); ok {
					env[k] = str
				} else {
					return nil, fmt.Errorf("environment[%s] must be a string, got %T", k, v)
				}
			}
			return env, nil
		}
	}
	return nil, nil
}

func evaluateHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filesStr, err := request.RequireString("files")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	fileFields := strings.Split(filesStr, ",")
	var files []string
	for _, f := range fileFields {
		trimmed := strings.TrimSpace(f)
		if trimmed != "" {
			files = append(files, trimmed)
		}
	}

	if len(files) == 0 {
		return mcp.NewToolResultError("No files provided"), nil
	}

	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	fileSystem, err := parseFileSystem(args)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	format := parseOptionalString(args, "format", "")

	env, err := parseEnvironment(args)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	testFS, err := createTestFS(fileSystem)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	output, err := bkl.Evaluate(testFS, files, "/", "", env, &format)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Evaluation failed: %v", err)), nil
	}

	response := map[string]interface{}{
		"files":     files,
		"format":    format,
		"output":    string(output),
		"operation": "evaluate",
	}

	if len(env) > 0 {
		response["environment"] = env
	}

	resultJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func diffHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	baseFile, err := request.RequireString("baseFile")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	targetFile, err := request.RequireString("targetFile")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	fileSystem, err := parseFileSystem(args)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	format := parseOptionalString(args, "format", "")

	testFS, err := createTestFS(fileSystem)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if format == "" {
		format = "yaml"
	}
	output, err := bkl.Diff(testFS, baseFile, targetFile, "/", "", &format)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Diff operation failed: %v", err)), nil
	}

	response := map[string]interface{}{
		"baseFile":   baseFile,
		"targetFile": targetFile,
		"format":     format,
		"output":     string(output),
		"operation":  "diff",
	}

	resultJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func intersectHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filesStr, err := request.RequireString("files")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	fileFields := strings.Split(filesStr, ",")
	var files []string
	for _, f := range fileFields {
		trimmed := strings.TrimSpace(f)
		if trimmed != "" {
			files = append(files, trimmed)
		}
	}

	if len(files) < 2 {
		return mcp.NewToolResultError("Intersect operation requires at least 2 files"), nil
	}

	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	fileSystem, err := parseFileSystem(args)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	format := parseOptionalString(args, "format", "")

	testFS, err := createTestFS(fileSystem)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if format == "" {
		format = "yaml"
	}
	output, err := bkl.Intersect(testFS, files, "/", "", &format)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Intersect operation failed: %v", err)), nil
	}

	response := map[string]interface{}{
		"files":     files,
		"format":    format,
		"output":    string(output),
		"operation": "intersect",
	}

	resultJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func requiredHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	file, err := request.RequireString("file")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	fileSystem, err := parseFileSystem(args)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	format := parseOptionalString(args, "format", "")

	testFS, err := createTestFS(fileSystem)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if format == "" {
		format = "yaml"
	}
	output, err := bkl.Required(testFS, file, "/", "", &format)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Required operation failed: %v", err)), nil
	}

	response := map[string]interface{}{
		"file":      file,
		"format":    format,
		"output":    string(output),
		"operation": "required",
	}

	resultJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(resultJSON)), nil
}
