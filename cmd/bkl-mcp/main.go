package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"sort"
	"strings"
	"testing/fstest"

	"github.com/gopatchy/bkl"
	"github.com/gopatchy/bkl/pkg/version"
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
		mcp.Description("Map of filename to file content. If not provided, uses actual filesystem in current directory"),
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
		mcp.WithString("source",
			mcp.Description("Source file for documentation (e.g., 'index', 'k8s'). Only applies to type='documentation'"),
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
		mcp.WithString("workingDir",
			mcp.Description("Working directory for file operations (default: current directory)"),
		),
		mcp.WithString("outputPath",
			mcp.Description("Optional path to write the output to (in addition to returning it)"),
		),
		mcp.WithString("sortPath",
			mcp.Description("Sort output documents by path (e.g. 'name' or 'metadata.priority')"),
		),
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
		mcp.WithString("selector",
			mcp.Description("Selector expression to match documents (e.g. 'metadata.name')"),
		),
		formatParam,
		fileSystemParam,
		mcp.WithString("workingDir",
			mcp.Description("Working directory for file operations (default: current directory)"),
		),
		mcp.WithString("outputPath",
			mcp.Description("Optional path to write the output to (in addition to returning it)"),
		),
	)
	mcpServer.AddTool(diffTool, diffHandler)

	intersectTool := mcp.NewTool("intersect",
		mcp.WithDescription("Generate the maximal base layer that the specified targets have in common"),
		mcp.WithString("files",
			mcp.Required(),
			mcp.Description("Comma-separated list of files to intersect (requires at least 2 files)"),
		),
		mcp.WithString("selector",
			mcp.Description("Selector expression to match documents (e.g. 'metadata.name')"),
		),
		formatParam,
		fileSystemParam,
		mcp.WithString("workingDir",
			mcp.Description("Working directory for file operations (default: current directory)"),
		),
		mcp.WithString("outputPath",
			mcp.Description("Optional path to write the output to (in addition to returning it)"),
		),
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
		mcp.WithString("workingDir",
			mcp.Description("Working directory for file operations (default: current directory)"),
		),
		mcp.WithString("outputPath",
			mcp.Description("Optional path to write the output to (in addition to returning it)"),
		),
	)
	mcpServer.AddTool(requiredTool, requiredHandler)

	versionTool := mcp.NewTool("version",
		mcp.WithDescription("Get version and build information for bkl"),
	)
	mcpServer.AddTool(versionTool, versionHandler)

	issuePromptTool := mcp.NewTool("issue_prompt",
		mcp.WithDescription("Get guidance for filing an issue with minimal reproduction case"),
	)
	mcpServer.AddTool(issuePromptTool, issuePromptHandler)

	convertToBklTool := mcp.NewTool("convert_to_bkl",
		mcp.WithDescription("Get guidance for converting YAML files to bkl format with layering"),
	)
	mcpServer.AddTool(convertToBklTool, convertToBklHandler)

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

	var allResults []map[string]any

	for _, section := range sections {
		score := 0
		details := map[string]any{}

		titleLower := strings.ToLower(section.Title)
		idLower := strings.ToLower(section.ID)

		titleMatches := countKeywordMatches(titleLower, normalizedKeywords)
		idMatches := countKeywordMatches(idLower, normalizedKeywords)
		sourceMatches := countKeywordMatches(section.Source, normalizedKeywords)

		score += titleMatches * 20
		score += idMatches * 15
		score += sourceMatches * 30

		matchingContent := []string{}
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
					matchingContent = append(matchingContent, content)
				}
			}
			if item.Example != nil {
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
						details["example_label"] = item.Code.Label
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
			result := map[string]any{
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
		details := map[string]any{}

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
			result := map[string]any{
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

	response := map[string]any{
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

	args, ok := request.Params.Arguments.(map[string]any)
	if !ok {
		args = map[string]any{}
	}
	source := parseOptionalString(args, "source", "")

	switch contentType {
	case "documentation":
		for _, section := range sections {
			if section.ID == id {
				if source != "" && section.Source != source {
					continue
				}
				sectionJSON, err := json.MarshalIndent(section, "", "  ")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				return mcp.NewToolResultText(string(sectionJSON)), nil
			}
		}
		if source != "" {
			return mcp.NewToolResultText(fmt.Sprintf("Documentation section '%s' not found in source '%s'", id, source)), nil
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

func parseFileSystem(args map[string]any) (map[string]string, error) {
	fileSystemRaw := args["fileSystem"]
	if fileSystemRaw == nil {
		return nil, nil
	}

	fileSystemMap, ok := fileSystemRaw.(map[string]any)
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

func parseOptionalString(args map[string]any, key string, defaultValue string) string {
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

func getFileSystem(fileSystem map[string]string, workingDir string) (fs.FS, error) {
	if fileSystem != nil {
		return createTestFS(fileSystem)
	}

	if workingDir == "" {
		workingDir = "."
	}
	return os.DirFS(workingDir), nil
}

func parseEnvironment(args map[string]any) (map[string]string, error) {
	if envRaw := args["environment"]; envRaw != nil {
		if envMap, ok := envRaw.(map[string]any); ok {
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

	args, ok := request.Params.Arguments.(map[string]any)
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

	workingDir := parseOptionalString(args, "workingDir", ".")

	fsys, err := getFileSystem(fileSystem, workingDir)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	rootPath := "."
	if fileSystem != nil {
		// For in-memory filesystem, use current directory as root
		// since files are already in the correct format
		workingDir = "."
	}

	sortPath := parseOptionalString(args, "sortPath", "")
	output, err := bkl.Evaluate(fsys, files, rootPath, workingDir, env, &format, sortPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Evaluation failed: %v", err)), nil
	}

	outputPath := parseOptionalString(args, "outputPath", "")
	if outputPath != "" {
		if err := os.WriteFile(outputPath, output, 0o644); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to write output to %s: %v", outputPath, err)), nil
		}
	}

	response := map[string]any{
		"files":     files,
		"format":    format,
		"output":    string(output),
		"operation": "evaluate",
	}

	if len(env) > 0 {
		response["environment"] = env
	}

	if outputPath != "" {
		response["outputPath"] = outputPath
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

	args, ok := request.Params.Arguments.(map[string]any)
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	fileSystem, err := parseFileSystem(args)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	format := parseOptionalString(args, "format", "")
	workingDir := parseOptionalString(args, "workingDir", ".")

	fsys, err := getFileSystem(fileSystem, workingDir)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	rootPath := "."
	if fileSystem != nil {
		// For in-memory filesystem, use current directory as root
		// since files are already in the correct format
		workingDir = "."
	}

	if format == "" {
		format = "yaml"
	}
	selector := parseOptionalString(args, "selector", "")
	output, err := bkl.Diff(fsys, baseFile, targetFile, rootPath, workingDir, selector, &format)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Diff operation failed: %v", err)), nil
	}

	outputPath := parseOptionalString(args, "outputPath", "")
	if outputPath != "" {
		if err := os.WriteFile(outputPath, output, 0o644); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to write output to %s: %v", outputPath, err)), nil
		}
	}

	response := map[string]any{
		"baseFile":   baseFile,
		"targetFile": targetFile,
		"format":     format,
		"output":     string(output),
		"operation":  "diff",
	}

	if outputPath != "" {
		response["outputPath"] = outputPath
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

	args, ok := request.Params.Arguments.(map[string]any)
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	fileSystem, err := parseFileSystem(args)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	format := parseOptionalString(args, "format", "")
	workingDir := parseOptionalString(args, "workingDir", ".")

	fsys, err := getFileSystem(fileSystem, workingDir)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	rootPath := "."
	if fileSystem != nil {
		// For in-memory filesystem, use current directory as root
		// since files are already in the correct format
		workingDir = "."
	}

	if format == "" {
		format = "yaml"
	}
	selector := parseOptionalString(args, "selector", "")
	output, err := bkl.Intersect(fsys, files, rootPath, workingDir, selector, &format)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Intersect operation failed: %v", err)), nil
	}

	outputPath := parseOptionalString(args, "outputPath", "")
	if outputPath != "" {
		if err := os.WriteFile(outputPath, output, 0o644); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to write output to %s: %v", outputPath, err)), nil
		}
	}

	response := map[string]any{
		"files":     files,
		"format":    format,
		"output":    string(output),
		"operation": "intersect",
	}

	if outputPath != "" {
		response["outputPath"] = outputPath
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

	args, ok := request.Params.Arguments.(map[string]any)
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	fileSystem, err := parseFileSystem(args)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	format := parseOptionalString(args, "format", "")
	workingDir := parseOptionalString(args, "workingDir", ".")

	fsys, err := getFileSystem(fileSystem, workingDir)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	rootPath := "."
	if fileSystem != nil {
		// For in-memory filesystem, use current directory as root
		// since files are already in the correct format
		workingDir = "."
	}

	if format == "" {
		format = "yaml"
	}
	output, err := bkl.Required(fsys, file, rootPath, workingDir, &format)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Required operation failed: %v", err)), nil
	}

	outputPath := parseOptionalString(args, "outputPath", "")
	if outputPath != "" {
		if err := os.WriteFile(outputPath, output, 0o644); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to write output to %s: %v", outputPath, err)), nil
		}
	}

	response := map[string]any{
		"file":      file,
		"format":    format,
		"output":    string(output),
		"operation": "required",
	}

	if outputPath != "" {
		response["outputPath"] = outputPath
	}

	resultJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func versionHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	bi := version.GetVersion()
	if bi == nil {
		return mcp.NewToolResultError("Failed to get build information"), nil
	}

	resultJSON, err := json.MarshalIndent(bi, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func issuePromptHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	prompt := `# Filing a bkl Issue - Steps

1. **Create a minimal reproduction case**:
   - Use the evaluate/diff/intersect/required tools to isolate the problem
   - Start with your full configuration that shows the issue
   - Progressively simplify while ensuring the issue still occurs

2. **Sanitize the configuration**:
   - Replace sensitive values with generic ones (e.g., "secret123" → "value1")
   - Use short, generic identifiers (e.g., "prod-api-key" → "a", "staging-db-host" → "b")
   - Keep the structure and issue intact while removing business context

3. **Get version information**:
   - Use the version tool to get build details
   - Include this in your issue report

4. **File the issue using GitHub CLI**:
   - Use ` + "`gh issue create`" + ` in the gopatchy/bkl repository
   - Include:
     - Clear description of expected vs actual behavior
     - Minimal reproduction case (configuration files)
     - Version information
     - Any error messages

Example workflow:
` + "```" + `bash
# Test your minimal case
mcp call bkl-mcp evaluate --files "test.yaml" --fileSystem '{"test.yaml": "your minimal config here"}'

# Get version info
mcp call bkl-mcp version

# File issue
gh issue create --repo gopatchy/bkl --title "Brief description" --body "..."
` + "```" + `

Tips for minimal reproductions:
- If the issue involves inheritance, include both parent and child files
- For $match issues, include the matching documents
- For encoding/decoding issues, show input and expected output
- Keep file contents as small as possible while reproducing the issue`

	return mcp.NewToolResultText(prompt), nil
}

func convertToBklHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	prompt := `# Converting YAML Files to bkl Format

This guide helps you convert a set of YAML files into bkl format with proper layering.

## Steps:

### 1. Determine Layer Ordering
- If layers represent different environments (prod, staging, dev):
  - Use production as the base layer (bottom)
  - Stack environments in order: prod → staging → dev
  - This shows each environment as differences from production
  - Encourages environments to stay similar to production

### 2. Find Common Base Values
For files at the same level (e.g., multiple services in prod):
` + "```" + `bash
# Find common values between original production configs
mcp call bkl-mcp intersect \
  --files "original.service1.prod.yaml,original.service2.prod.yaml" \
  --fileSystem '{
    "original.service1.prod.yaml": "...",
    "original.service2.prod.yaml": "..."
  }'
` + "```" + `

### 3. Generate Layer Differences
Once you have the base layer, create upper layers using diff:
` + "```" + `bash
# Generate staging differences from prod base
mcp call bkl-mcp diff \
  --baseFile "converted.service1.yaml" \
  --targetFile "original.service1.staging.yaml" \
  --fileSystem '{
    "converted.service1.yaml": "...",
    "original.service1.staging.yaml": "..."
  }'
` + "```" + `

### 4. Optimize with Patterns
When many values follow patterns across environments:
- Use string interpolation ($"") to derive values from variables
- Use $merge when combining multiple configuration sources

Example:
` + "```" + `yaml
# converted.service1.yaml
environment: prod
database_url: $"postgres://db.{environment}.example.com"
api_endpoint: $"https://api.{environment}.example.com"
cache_ttl: 3600

# converted.service1.prod.yaml
# Empty or minimal - production uses base values

# converted.service1.prod.staging.yaml
environment: staging  # Changes all interpolated values
cache_ttl: 300       # Override specific value
` + "```" + `

### 5. Handle Secrets and Required Fields
Mark fields based on how they're managed:
` + "```" + `yaml
# converted.service1.yaml
# For secrets from a secret store:
api_key: $env:API_KEY
database_password: $env:DB_PASSWORD

# For values that must be manually configured per environment:
region: $required
cluster_name: $required
` + "```" + `

### 6. Validate Results
Compare the evaluated output with original files:
` + "```" + `bash
# Test that converted layers produce original staging config
mcp call bkl-mcp evaluate \
  --files "converted.service1.yaml,converted.service1.prod.yaml,converted.service1.prod.staging.yaml" \
  --fileSystem '{
    "converted.service1.yaml": "...",
    "converted.service1.prod.yaml": "...",
    "converted.service1.prod.staging.yaml": "..."
  }' \
  --format yaml
` + "```" + `

### 7. Iterate and Refine
- If outputs don't match, adjust layer content
- Consider moving common patterns to base layer
- Use $parent for explicit inheritance when needed
- Ensure layers are human-readable and maintainable

## Best Practices:
- Keep base layer comprehensive but not overly specific
- Use environment variables for runtime configuration
- Group related configuration with meaningful structure
- Document layer relationships and dependencies
- Test each layer combination thoroughly

## Example Workflow:
1. Start with: original.service1.prod.yaml, original.service1.staging.yaml, original.service1.dev.yaml
2. Use production as base → copy original.service1.prod.yaml to converted.service1.yaml
3. Use diff to create upper layers:
   - converted.service1.prod.yaml (usually empty since base = prod)
   - converted.service1.prod.staging.yaml (staging differences from prod)
   - converted.service1.prod.staging.dev.yaml (dev differences from staging)
4. Add string interpolation patterns for environment-specific values
5. Mark secrets and keys as $env:SECRET_NAME if using a secret store, or $required if manually configured
6. Validate each combination produces original output
7. Final structure:
   - converted.service1.yaml (production configuration as base)
   - converted.service1.prod.yaml (usually empty)
   - converted.service1.prod.staging.yaml (staging differences)
   - converted.service1.prod.staging.dev.yaml (development differences)

Note: Use intersect when multiple services need a shared base layer, not for single service environment layering.

## Kubernetes-Specific Notes*

*For converting Kubernetes manifests to bkl format:

- **Clean manifests**: Use kubectl-neat to remove server-generated fields: ` + "`kubectl get deployment -o yaml | kubectl neat`" + `
- **Helm templates**: Evaluate helm charts first: ` + "`helm template myrelease mychart > original.yaml`" + `
- **Use filename layering**: ` + "`app.yaml`" + ` → ` + "`app.staging.yaml`" + ` → ` + "`app.staging.dev.yaml`" + ` (NOT $parent)
- **Common patterns**:
  - Namespaces differ by environment: ` + "`namespace: $\"myapp-{environment}\"`" + `
  - Image tags from env vars: ` + "`image: $\"myregistry.com/myapp:{env:IMAGE_TAG}\"`" + `
  - Progressive resource limits: prod=1Gi, staging=512Mi, dev=256Mi
  - Service URLs change: ` + "`$\"postgres.{namespace}.svc.cluster.local\"`" + `
- **File organization**: Split by component (api.yaml, web.yaml, database.yaml)
- **Remove resources**: Use ` + "`$output: false`" + ` to exclude from specific environments
- **Secrets**: Use ` + "`$env:SECRET_NAME`" + ` or Kubernetes secretKeyRef
- **Testing**: Use bkl-mcp evaluate (NOT the CLI) with environment variables`

	return mcp.NewToolResultText(prompt), nil
}
