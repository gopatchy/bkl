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
	"github.com/gopatchy/bkl/internal/utils"
	"github.com/gopatchy/bkl/pkg/version"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
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

	compareFilesTool := mcp.NewTool("compare_files",
		mcp.WithDescription("Evaluate two bkl files and show text differences between their outputs"),
		mcp.WithString("file1",
			mcp.Required(),
			mcp.Description("First file path to evaluate"),
		),
		mcp.WithString("file2",
			mcp.Required(),
			mcp.Description("Second file path to evaluate"),
		),
		formatParam,
		fileSystemParam,
		mcp.WithObject("environment",
			mcp.Description("Environment variables as key-value pairs"),
		),
		mcp.WithString("sortPath",
			mcp.Description("Sort output documents by path (e.g. 'name' or 'metadata.priority')"),
		),
	)
	mcpServer.AddTool(compareFilesTool, compareFilesHandler)

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

func getFileSystem(fileSystem map[string]string) (fs.FS, error) {
	if fileSystem != nil {
		return createTestFS(fileSystem)
	}
	return os.DirFS("/"), nil
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

func determineFormatWithPaths(explicitFormat string, outputPath string, inputPaths []string) string {
	if explicitFormat != "" {
		return explicitFormat
	}

	if outputPath != "" {
		if ext := utils.Ext(outputPath); ext != "" {
			return ext
		}
	}

	for _, path := range inputPaths {
		if ext := utils.Ext(path); ext != "" {
			return ext
		}
	}

	return ""
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
	outputPath := parseOptionalString(args, "outputPath", "")

	env, err := parseEnvironment(args)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	workingDir := ""
	if fileSystem != nil {
		workingDir = "/"
	}

	fsys, err := getFileSystem(fileSystem)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	finalFormat := determineFormatWithPaths(format, outputPath, files)

	sortPath := parseOptionalString(args, "sortPath", "")
	output, err := bkl.Evaluate(fsys, files, "/", workingDir, env, &finalFormat, sortPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Evaluation failed: %v", err)), nil
	}

	if outputPath != "" {
		if err := os.WriteFile(outputPath, output, 0o644); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to write output to %s: %v", outputPath, err)), nil
		}
	}

	response := map[string]any{
		"files":     files,
		"format":    finalFormat,
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
	outputPath := parseOptionalString(args, "outputPath", "")
	workingDir := ""
	if fileSystem != nil {
		workingDir = "/"
	}

	fsys, err := getFileSystem(fileSystem)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	finalFormat := determineFormatWithPaths(format, outputPath, []string{baseFile, targetFile})
	if finalFormat == "" {
		finalFormat = "yaml"
	}
	selector := parseOptionalString(args, "selector", "")
	output, err := bkl.Diff(fsys, baseFile, targetFile, "/", workingDir, selector, &finalFormat)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Diff operation failed: %v", err)), nil
	}

	if outputPath != "" {
		if err := os.WriteFile(outputPath, output, 0o644); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to write output to %s: %v", outputPath, err)), nil
		}
	}

	response := map[string]any{
		"baseFile":   baseFile,
		"targetFile": targetFile,
		"format":     finalFormat,
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
	outputPath := parseOptionalString(args, "outputPath", "")
	workingDir := ""
	if fileSystem != nil {
		workingDir = "/"
	}

	fsys, err := getFileSystem(fileSystem)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	finalFormat := determineFormatWithPaths(format, outputPath, files)
	if finalFormat == "" {
		finalFormat = "yaml"
	}
	selector := parseOptionalString(args, "selector", "")
	output, err := bkl.Intersect(fsys, files, "/", workingDir, selector, &finalFormat)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Intersect operation failed: %v", err)), nil
	}

	if outputPath != "" {
		if err := os.WriteFile(outputPath, output, 0o644); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to write output to %s: %v", outputPath, err)), nil
		}
	}

	response := map[string]any{
		"files":     files,
		"format":    finalFormat,
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
	outputPath := parseOptionalString(args, "outputPath", "")
	workingDir := ""
	if fileSystem != nil {
		workingDir = "/"
	}

	fsys, err := getFileSystem(fileSystem)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	finalFormat := determineFormatWithPaths(format, outputPath, []string{file})
	if finalFormat == "" {
		finalFormat = "yaml"
	}
	output, err := bkl.Required(fsys, file, "/", workingDir, &finalFormat)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Required operation failed: %v", err)), nil
	}

	if outputPath != "" {
		if err := os.WriteFile(outputPath, output, 0o644); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to write output to %s: %v", outputPath, err)), nil
		}
	}

	response := map[string]any{
		"file":      file,
		"format":    finalFormat,
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
	prompt := `# Converting Kubernetes YAML files to bkl format

These instructions help you convert a set of Kubernetes YAML files into bkl format with proper layering. They can also be used for non-Kubernetes YAML files.

Update your Todos to do these steps in order:
1) Find all input files
2) Prep files for bkl (see mcp__bkl-mcp__get type="documentation" id="prep" source="k8s")
3) Validate prepped files (see mcp__bkl-mcp__get type="documentation" id="prep-validate" source="k8s")
4) Determine a target layout (see mcp__bkl-mcp__get type="documentation" id="plan" source="k8s")
5) Create base layers (see mcp__bkl-mcp__get type="documentation" id="base" source="k8s")
6) Create remaining layers (see mcp__bkl-mcp__get type="documentation" id="api-service" source="k8s")
7) Validate all bkl leaf layers against the original configs (see mcp__bkl-mcp__get type="documentation" id="api-service" source="k8s")

Tools:
* To query documentation: mcp__bkl-mcp__query keywords="repeat,list,iteration"
* Instead of bkl, use: mcp__bkl-mcp__evaluate files="prep/prod/namespace.yaml" outputPath="bkl/namespace.yaml"
* Instead of bkli, use: mcp__bkl-mcp__intersect files="prep/prod/api-service.yaml,prep/prod/web-service.yaml" outputPath="bkl/base.yaml" selector="kind"
* Instead of bkld, use: mcp__bkl-mcp__diff baseFile="bkl/namespace.yaml" targetFile="prep/staging/namespace.yaml" outputPath="bkl/namespace.staging.yaml" selector="kind"
* Instead of diff <(bkl ...) <(bkl ...), use: mcp__bkl-mcp__compare_files file1="original/prod/namespace.yaml" file2="prep/prod//namespace.yaml"

Rules:
* ALWAYS consider & examine EVERY file during the prep step
* ALWAYS convert every list that might need overriding (containers, env, ports, etc.) to a map
* ALWAYS stack environments: dev on staging on prod
* ALWAYS name files to indicate their layering: layer1.layer2.layer3.yaml
* ALWAYS use mcp__bkl-mcp__evaluate to evaluate EACH file after you create it, and fix any errors before continuing
* NEVER put the environment name in an environment variable
* NEVER use $parent to specify inheritance
* NEVER put bkl files in multiple directories; put them all in a single directory
* NEVER use mcp__bkl-mcp__evaluate with multiple files at once
* NEVER use external scripts to split, alter, or parse files
`

	return mcp.NewToolResultText(prompt), nil
}

func compareFilesHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	file1, err := request.RequireString("file1")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	file2, err := request.RequireString("file2")
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

	env, err := parseEnvironment(args)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	workingDir := ""
	if fileSystem != nil {
		workingDir = "/"
	}

	fsys, err := getFileSystem(fileSystem)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	finalFormat := determineFormatWithPaths(format, "", []string{file1, file2})
	sortPath := parseOptionalString(args, "sortPath", "")

	output1, err := bkl.Evaluate(fsys, []string{file1}, "/", workingDir, env, &finalFormat, sortPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to evaluate %s: %v", file1, err)), nil
	}

	output2, err := bkl.Evaluate(fsys, []string{file2}, "/", workingDir, env, &finalFormat, sortPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to evaluate %s: %v", file2, err)), nil
	}

	edits := myers.ComputeEdits(span.URIFromPath(file1), string(output1), string(output2))
	unified := fmt.Sprint(gotextdiff.ToUnified(file1, file2, string(output1), edits))

	response := map[string]any{
		"file1":     file1,
		"file2":     file2,
		"format":    finalFormat,
		"diff":      unified,
		"operation": "compare_files",
	}

	if len(env) > 0 {
		response["environment"] = env
	}

	if sortPath != "" {
		response["sortPath"] = sortPath
	}

	jsonResult, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonResult)), nil
}
