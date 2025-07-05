package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"runtime/debug"
	"sort"
	"strings"
	"testing/fstest"

	"github.com/gopatchy/bkl"
	"github.com/gopatchy/bkl/internal/utils"
	"github.com/gopatchy/bkl/pkg/version"
	"github.com/gopatchy/taskcp"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var (
	tests       map[string]*bkl.TestCase
	sections    []bkl.DocSection
	taskService *taskcp.Service
)

type queryArgs struct {
	Keywords string `json:"keywords"`
}

type getArgs struct {
	Type   string `json:"type"`
	ID     string `json:"id"`
	Source string `json:"source,omitempty"`
}

type evaluateArgs struct {
	Files         string            `json:"files,omitempty"`
	Directory     string            `json:"directory,omitempty"`
	Pattern       string            `json:"pattern,omitempty"`
	IncludeOutput *bool             `json:"includeOutput,omitempty"`
	Format        string            `json:"format,omitempty"`
	Environment   map[string]string `json:"environment,omitempty"`
	FileSystem    map[string]string `json:"fileSystem,omitempty"`
	OutputPath    string            `json:"outputPath,omitempty"`
	SortPath      string            `json:"sortPath,omitempty"`
}

type diffArgs struct {
	BaseFile   string            `json:"baseFile"`
	TargetFile string            `json:"targetFile"`
	Selector   string            `json:"selector,omitempty"`
	Format     string            `json:"format,omitempty"`
	FileSystem map[string]string `json:"fileSystem,omitempty"`
	OutputPath string            `json:"outputPath,omitempty"`
}

type intersectArgs struct {
	Files      string            `json:"files"`
	Selector   string            `json:"selector,omitempty"`
	Format     string            `json:"format,omitempty"`
	FileSystem map[string]string `json:"fileSystem,omitempty"`
	OutputPath string            `json:"outputPath,omitempty"`
}

type requiredArgs struct {
	File       string            `json:"file"`
	Format     string            `json:"format,omitempty"`
	FileSystem map[string]string `json:"fileSystem,omitempty"`
	OutputPath string            `json:"outputPath,omitempty"`
}

type compareArgs struct {
	File1       string            `json:"file1"`
	File2       string            `json:"file2"`
	Format      string            `json:"format,omitempty"`
	FileSystem  map[string]string `json:"fileSystem,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	SortPath    string            `json:"sortPath,omitempty"`
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
	Features       []string `json:"features,omitempty"`
}

type queryResponse struct {
	Keywords []string      `json:"keywords"`
	Results  []queryResult `json:"results"`
	Count    int           `json:"count"`
}

type evaluateResponse struct {
	Files        []string          `json:"files,omitempty"`
	Directory    string            `json:"directory,omitempty"`
	Pattern      string            `json:"pattern,omitempty"`
	TotalFiles   int               `json:"totalFiles,omitempty"`
	SuccessCount int               `json:"successCount,omitempty"`
	ErrorCount   int               `json:"errorCount,omitempty"`
	Results      []evaluateResult  `json:"results,omitempty"`
	Format       string            `json:"format"`
	Output       string            `json:"output"`
	Operation    string            `json:"operation"`
	Environment  map[string]string `json:"environment,omitempty"`
	OutputPath   string            `json:"outputPath,omitempty"`
}

type evaluateResult struct {
	Path   string `json:"path"`
	Error  string `json:"error,omitempty"`
	Output string `json:"output,omitempty"`
}

type diffResponse struct {
	BaseFile   string `json:"baseFile"`
	TargetFile string `json:"targetFile"`
	Format     string `json:"format"`
	Output     string `json:"output"`
	Operation  string `json:"operation"`
	OutputPath string `json:"outputPath,omitempty"`
}

type intersectResponse struct {
	Files      []string `json:"files"`
	Format     string   `json:"format"`
	Output     string   `json:"output"`
	Operation  string   `json:"operation"`
	OutputPath string   `json:"outputPath,omitempty"`
}

type requiredResponse struct {
	File       string `json:"file"`
	Format     string `json:"format"`
	Output     string `json:"output"`
	Operation  string `json:"operation"`
	OutputPath string `json:"outputPath,omitempty"`
}

type compareResponse struct {
	File1       string            `json:"file1"`
	File2       string            `json:"file2"`
	Format      string            `json:"format"`
	Diff        string            `json:"diff"`
	Operation   string            `json:"operation"`
	Environment map[string]string `json:"environment,omitempty"`
	SortPath    string            `json:"sortPath,omitempty"`
}

type promptResponse struct {
	Prompt string `json:"prompt"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type getResponse struct {
	Documentation *bkl.DocSection `json:"documentation,omitempty"`
	Test          *bkl.TestCase   `json:"test,omitempty"`
}

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

type HandlerFunc[TArgs any, TResponse any] func(ctx context.Context, args TArgs) (*TResponse, error)

func wrapHandler[TArgs any, TResponse any](handler HandlerFunc[TArgs, TResponse]) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args TArgs
		if err := request.BindArguments(&args); err != nil {
			errorJSON, _ := json.Marshal(errorResponse{Error: err.Error()})
			return mcp.NewToolResultText(string(errorJSON)), nil
		}

		response, err := handler(ctx, args)
		if err != nil {
			errorJSON, _ := json.Marshal(errorResponse{Error: err.Error()})
			return mcp.NewToolResultText(string(errorJSON)), nil
		}

		resultJSON, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			errorJSON, _ := json.Marshal(errorResponse{Error: err.Error()})
			return mcp.NewToolResultText(string(errorJSON)), nil
		}

		return mcp.NewToolResultText(string(resultJSON)), nil
	}
}

func main() {
	if err := loadData(); err != nil {
		log.Fatalf("Failed to load data: %v", err)
	}

	taskService = taskcp.New()

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
	mcpServer.AddTool(queryTool, wrapHandler(queryHandler))

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
	mcpServer.AddTool(getTool, wrapHandler(getHandler))

	evaluateTool := mcp.NewTool("evaluate",
		mcp.WithDescription("Evaluate bkl files with given environment and return results"),
		mcp.WithString("files",
			mcp.Description("Comma-separated list of files to evaluate (relative paths). Leave empty when using directory parameter."),
		),
		mcp.WithString("directory",
			mcp.Description("Directory path to evaluate all files within (alternative to files parameter)"),
		),
		mcp.WithString("pattern",
			mcp.Description("File pattern to match when using directory mode (e.g. '*.yaml', '*.bkl')"),
		),
		mcp.WithBoolean("includeOutput",
			mcp.Description("Include evaluated output for successful files when in directory mode (default: true)"),
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
	mcpServer.AddTool(evaluateTool, wrapHandler(evaluateHandler))

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
	mcpServer.AddTool(diffTool, wrapHandler(diffHandler))

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
	mcpServer.AddTool(intersectTool, wrapHandler(intersectHandler))

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
	mcpServer.AddTool(requiredTool, wrapHandler(requiredHandler))

	versionTool := mcp.NewTool("version",
		mcp.WithDescription("Get version and build information for bkl"),
	)
	mcpServer.AddTool(versionTool, wrapHandler(versionHandler))

	issuePromptTool := mcp.NewTool("issue_prompt",
		mcp.WithDescription("Get guidance for filing an issue with minimal reproduction case"),
	)
	mcpServer.AddTool(issuePromptTool, wrapHandler(issuePromptHandler))

	convertToBklTool := mcp.NewTool("convert_to_bkl",
		mcp.WithDescription("Get guidance for converting YAML files to bkl format with layering"),
	)
	mcpServer.AddTool(convertToBklTool, wrapHandler(convertToBklHandler))

	compareTool := mcp.NewTool("compare",
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
	mcpServer.AddTool(compareTool, wrapHandler(compareHandler))

	if err := taskcp.RegisterMCPTools(mcpServer, taskService); err != nil {
		log.Fatalf("Failed to register taskcp tools: %v", err)
	}

	if err := server.ServeStdio(mcpServer); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func queryHandler(ctx context.Context, args queryArgs) (*queryResponse, error) {
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

	for _, section := range sections {
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
			}
			allResults = append(allResults, result)
		}
	}

	for name, test := range tests {
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

func getHandler(ctx context.Context, args getArgs) (*getResponse, error) {
	switch args.Type {
	case "documentation":
		for _, section := range sections {
			if section.ID == args.ID {
				if args.Source != "" && section.Source != args.Source {
					continue
				}
				return &getResponse{Documentation: &section}, nil
			}
		}
		if args.Source != "" {
			return nil, fmt.Errorf("documentation section '%s' not found in source '%s'", args.ID, args.Source)
		}
		return nil, fmt.Errorf("documentation section '%s' not found", args.ID)

	case "test":
		test, exists := tests[args.ID]
		if !exists {
			return nil, fmt.Errorf("test '%s' not found", args.ID)
		}
		return &getResponse{Test: test}, nil

	default:
		return nil, fmt.Errorf("invalid type '%s'. Must be 'documentation' or 'test'", args.Type)
	}
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

func evaluateHandler(ctx context.Context, args evaluateArgs) (*evaluateResponse, error) {
	if args.Directory != "" && args.Files != "" {
		return nil, fmt.Errorf("cannot specify both files and directory parameters")
	}

	if args.Directory == "" && args.Files == "" {
		return nil, fmt.Errorf("must specify either files or directory parameter")
	}

	workingDir := ""
	if args.FileSystem != nil {
		workingDir = "/"
	}

	fsys, err := getFileSystem(args.FileSystem)
	if err != nil {
		return nil, err
	}

	if args.Directory != "" {
		includeOutput := true
		if args.IncludeOutput != nil {
			includeOutput = *args.IncludeOutput
		}

		results, err := bkl.EvaluateTree(fsys, args.Directory, args.Pattern, args.Environment, &args.Format)
		if err != nil {
			return nil, fmt.Errorf("directory evaluation failed: %v", err)
		}

		successCount, errorCount := 0, 0
		for _, result := range results {
			if result.Error == nil {
				successCount++
			} else {
				errorCount++
			}
		}

		finalResults := []evaluateResult{}
		for _, result := range results {
			r := evaluateResult{
				Path: result.Path,
			}
			if result.Error != nil {
				r.Error = result.Error.Error()
			}
			if includeOutput && result.Output != "" {
				r.Output = result.Output
			}
			finalResults = append(finalResults, r)
		}

		return &evaluateResponse{
			Directory:    args.Directory,
			Pattern:      args.Pattern,
			TotalFiles:   len(results),
			SuccessCount: successCount,
			ErrorCount:   errorCount,
			Results:      finalResults,
			Operation:    "evaluate_tree",
			Environment:  args.Environment,
		}, nil
	}

	fileFields := strings.Split(args.Files, ",")
	files := []string{}
	for _, f := range fileFields {
		trimmed := strings.TrimSpace(f)
		if trimmed != "" {
			files = append(files, trimmed)
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files provided")
	}

	finalFormat := determineFormatWithPaths(args.Format, args.OutputPath, files)

	output, err := bkl.Evaluate(fsys, files, "/", workingDir, args.Environment, &finalFormat, args.SortPath)
	if err != nil {
		return nil, fmt.Errorf("evaluation failed: %v", err)
	}

	if args.OutputPath != "" {
		if err := os.WriteFile(args.OutputPath, output, 0o644); err != nil {
			return nil, fmt.Errorf("failed to write output to %s: %v", args.OutputPath, err)
		}
	}

	return &evaluateResponse{
		Files:       files,
		Format:      finalFormat,
		Output:      string(output),
		Operation:   "evaluate",
		Environment: args.Environment,
		OutputPath:  args.OutputPath,
	}, nil
}

func diffHandler(ctx context.Context, args diffArgs) (*diffResponse, error) {
	workingDir := ""
	if args.FileSystem != nil {
		workingDir = "/"
	}

	fsys, err := getFileSystem(args.FileSystem)
	if err != nil {
		return nil, err
	}

	finalFormat := determineFormatWithPaths(args.Format, args.OutputPath, []string{args.BaseFile, args.TargetFile})
	if finalFormat == "" {
		finalFormat = "yaml"
	}
	output, err := bkl.Diff(fsys, args.BaseFile, args.TargetFile, "/", workingDir, args.Selector, &finalFormat)
	if err != nil {
		return nil, fmt.Errorf("diff operation failed: %v", err)
	}

	if args.OutputPath != "" {
		if err := os.WriteFile(args.OutputPath, output, 0o644); err != nil {
			return nil, fmt.Errorf("failed to write output to %s: %v", args.OutputPath, err)
		}
	}

	return &diffResponse{
		BaseFile:   args.BaseFile,
		TargetFile: args.TargetFile,
		Format:     finalFormat,
		Output:     string(output),
		Operation:  "diff",
		OutputPath: args.OutputPath,
	}, nil
}

func intersectHandler(ctx context.Context, args intersectArgs) (*intersectResponse, error) {
	fileFields := strings.Split(args.Files, ",")
	files := []string{}
	for _, f := range fileFields {
		trimmed := strings.TrimSpace(f)
		if trimmed != "" {
			files = append(files, trimmed)
		}
	}

	if len(files) < 2 {
		return nil, fmt.Errorf("intersect operation requires at least 2 files")
	}
	workingDir := ""
	if args.FileSystem != nil {
		workingDir = "/"
	}

	fsys, err := getFileSystem(args.FileSystem)
	if err != nil {
		return nil, err
	}

	finalFormat := determineFormatWithPaths(args.Format, args.OutputPath, files)
	if finalFormat == "" {
		finalFormat = "yaml"
	}
	output, err := bkl.Intersect(fsys, files, "/", workingDir, args.Selector, &finalFormat)
	if err != nil {
		return nil, fmt.Errorf("intersect operation failed: %v", err)
	}

	if args.OutputPath != "" {
		if err := os.WriteFile(args.OutputPath, output, 0o644); err != nil {
			return nil, fmt.Errorf("failed to write output to %s: %v", args.OutputPath, err)
		}
	}

	return &intersectResponse{
		Files:      files,
		Format:     finalFormat,
		Output:     string(output),
		Operation:  "intersect",
		OutputPath: args.OutputPath,
	}, nil
}

func requiredHandler(ctx context.Context, args requiredArgs) (*requiredResponse, error) {
	workingDir := ""
	if args.FileSystem != nil {
		workingDir = "/"
	}

	fsys, err := getFileSystem(args.FileSystem)
	if err != nil {
		return nil, err
	}

	finalFormat := determineFormatWithPaths(args.Format, args.OutputPath, []string{args.File})
	if finalFormat == "" {
		finalFormat = "yaml"
	}
	output, err := bkl.Required(fsys, args.File, "/", workingDir, &finalFormat)
	if err != nil {
		return nil, fmt.Errorf("required operation failed: %v", err)
	}

	if args.OutputPath != "" {
		if err := os.WriteFile(args.OutputPath, output, 0o644); err != nil {
			return nil, fmt.Errorf("failed to write output to %s: %v", args.OutputPath, err)
		}
	}

	return &requiredResponse{
		File:       args.File,
		Format:     finalFormat,
		Output:     string(output),
		Operation:  "required",
		OutputPath: args.OutputPath,
	}, nil
}

func versionHandler(ctx context.Context, args struct{}) (*debug.BuildInfo, error) {
	bi := version.GetVersion()
	if bi == nil {
		return nil, fmt.Errorf("failed to get build information")
	}
	return bi, nil
}

func issuePromptHandler(ctx context.Context, args struct{}) (*promptResponse, error) {
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

	return &promptResponse{Prompt: prompt}, nil
}

func convertToBklHandler(ctx context.Context, args struct{}) (*promptResponse, error) {
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
* To validate a directory tree: mcp__bkl-mcp__validate_directory directory="prep" pattern="*.yaml"
* Instead of bkl, use: mcp__bkl-mcp__evaluate files="prep/prod/namespace.yaml" outputPath="bkl/namespace.yaml"
* Instead of bkli, use: mcp__bkl-mcp__intersect files="prep/prod/api-service.yaml,prep/prod/web-service.yaml" outputPath="bkl/base.yaml" selector="kind"
* Instead of bkld, use: mcp__bkl-mcp__diff baseFile="bkl/namespace.yaml" targetFile="prep/staging/namespace.yaml" outputPath="bkl/namespace.staging.yaml" selector="kind"
* Instead of bklc, use: mcp__bkl-mcp__compare file1="original/prod/namespace.yaml" file2="prep/prod/namespace.yaml"

Rules:
* ALWAYS consider & examine EVERY file during the prep step
* ALWAYS convert every list that might need overriding (containers, env, ports, etc.) to a map
* ALWAYS stack environments: dev on staging on prod
* ALWAYS name files to indicate their layering: layer1.layer2.layer3.yaml
* ALWAYS use mcp__bkl-mcp__evaluate to evaluate EACH file after you create it, and fix any errors before continuing. VALIDATE VALIDATE VALIDATE
* NEVER put the environment name in an environment variable
* NEVER use $parent to specify inheritance
* NEVER put bkl files in multiple directories; put them all in a single directory
* NEVER use mcp__bkl-mcp__evaluate with multiple files at once
* NEVER use external scripts to split, alter, or parse files
`

	return &promptResponse{Prompt: prompt}, nil
}

func compareHandler(ctx context.Context, args compareArgs) (*compareResponse, error) {
	workingDir := ""
	if args.FileSystem != nil {
		workingDir = "/"
	}

	fsys, err := getFileSystem(args.FileSystem)
	if err != nil {
		return nil, err
	}

	finalFormat := determineFormatWithPaths(args.Format, "", []string{args.File1, args.File2})

	result, err := bkl.Compare(fsys, args.File1, args.File2, "/", workingDir, args.Environment, &finalFormat, args.SortPath)
	if err != nil {
		return nil, err
	}

	return &compareResponse{
		File1:       result.File1,
		File2:       result.File2,
		Format:      result.Format,
		Diff:        result.Diff,
		Operation:   "compare",
		Environment: result.Environment,
		SortPath:    result.SortPath,
	}, nil
}
