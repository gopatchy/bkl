package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing/fstest"

	"github.com/gopatchy/bkl"
	"github.com/gopatchy/taskcp"
)

func (s *Server) convertHelmToPlain(files []string) ([]string, error) {
	chartDir := ""
	hasChart := false
	var valuesFiles []string

	for _, file := range files {
		base := filepath.Base(file)
		if base == "Chart.yaml" {
			hasChart = true
			chartDir = filepath.Dir(file)
		} else if strings.HasPrefix(base, "values") && (strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".yml")) {
			valuesFiles = append(valuesFiles, file)
			if base == "values.yaml" || base == "values.yml" {
				hasChart = true
			}
		}
	}

	if !hasChart {
		return nil, nil
	}

	if chartDir == "" && len(files) > 0 {
		chartDir = filepath.Dir(files[0])
	}

	if err := os.MkdirAll("plain", 0o755); err != nil {
		return nil, fmt.Errorf("failed to create plain directory: %w", err)
	}

	var plainFiles []string

	if len(valuesFiles) == 0 {
		valuesFiles = append(valuesFiles, "")
	}

	hasEnvSpecificFiles := false
	for _, valuesFile := range valuesFiles {
		base := filepath.Base(valuesFile)
		if base != "values.yaml" && base != "values.yml" {
			hasEnvSpecificFiles = true
			break
		}
	}

	for _, valuesFile := range valuesFiles {
		base := filepath.Base(valuesFile)
		if hasEnvSpecificFiles && (base == "values.yaml" || base == "values.yml") {
			continue
		}

		env := s.getEnvFromValuesFile(valuesFile)
		files, err := s.runHelmTemplate(chartDir, valuesFile, env)
		if err != nil {
			return nil, err
		}
		plainFiles = append(plainFiles, files...)
	}

	return plainFiles, nil
}

func (s *Server) getEnvFromValuesFile(valuesFile string) string {
	if valuesFile == "" {
		return "base"
	}

	base := filepath.Base(valuesFile)
	if base == "values.yaml" || base == "values.yml" {
		return "base"
	}

	nameWithoutExt := strings.TrimSuffix(strings.TrimSuffix(base, ".yaml"), ".yml")
	if e, found := strings.CutPrefix(nameWithoutExt, "values."); found && e != "" {
		return e
	}
	if e, found := strings.CutPrefix(nameWithoutExt, "values-"); found && e != "" {
		return e
	}

	return "base"
}

func (s *Server) runHelmTemplate(chartDir, valuesFile, env string) ([]string, error) {
	outputDir := filepath.Join("plain", env)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	args := []string{"template", "release", chartDir, "--output-dir", outputDir}
	if valuesFile != "" {
		args = append(args, "-f", valuesFile)
	}

	cmd := exec.Command("helm", args...)
	if err := cmd.Run(); err != nil {
		if valuesFile != "" {
			return nil, fmt.Errorf("helm template failed for %s: %w", valuesFile, err)
		}
		return nil, fmt.Errorf("helm template failed: %w", err)
	}

	files, err := s.findGeneratedFiles(outputDir)
	if err != nil {
		if valuesFile != "" {
			return nil, fmt.Errorf("failed to find generated files for %s: %w", valuesFile, err)
		}
		return nil, fmt.Errorf("failed to find generated files: %w", err)
	}

	return files, nil
}

func (s *Server) findGeneratedFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", dir, err)
	}
	return files, nil
}

func (s *Server) convertKustomizeToPlain(files []string) ([]string, error) {
	kustomizeDir := ""
	hasKustomization := false
	for _, file := range files {
		base := filepath.Base(file)
		if base == "kustomization.yaml" || base == "kustomization.yml" {
			hasKustomization = true
			kustomizeDir = filepath.Dir(file)
			break
		}
	}

	if !hasKustomization {
		return nil, nil
	}

	if kustomizeDir == "" && len(files) > 0 {
		kustomizeDir = filepath.Dir(files[0])
	}

	if err := os.MkdirAll("plain", 0o755); err != nil {
		return nil, fmt.Errorf("failed to create plain directory: %w", err)
	}

	outputFile := "plain/kustomize-output.yaml"

	cmd := exec.Command("kubectl", "kustomize", kustomizeDir)
	output, err := cmd.Output()
	if err != nil {
		cmd = exec.Command("kustomize", "build", kustomizeDir)
		output, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("kustomize build failed: %w", err)
		}
	}

	if err := os.WriteFile(outputFile, output, 0o644); err != nil {
		return nil, fmt.Errorf("failed to write kustomize output: %w", err)
	}

	return []string{outputFile}, nil
}

func (s *Server) convertToBklHandler(ctx context.Context, args struct{}) (*promptResponse, error) {
	p := s.taskService.AddProject()

	p.AddNextTask().
		WithTitle("Find configuration files").
		WithInstructions(`
* Find all the configuration files you need to convert.
* Call: {SUCCESS_PROMPT}
* Set result to a JSON encoding in the following format:
	{
		"files": [
			"path/to/file1.yaml",
			"path/to/file2.yaml"
		]
	}
`).
		Then(func(t *taskcp.Task) error {
			return s.convertToBklOnFiles(p, t)
		})

	t, err := p.PopNextTask()
	if err != nil {
		return nil, err
	}

	return &promptResponse{
		Prompt: `# Converting Kubernetes configuration files to bkl format

* I'll walk you through this process step by step.
* Follow EXACTLY these steps -- do not attempt to do the conversion yourself or follow any steps that are not EXACTLY what I tell you to do.
* DO NOT invent a TODO list -- just execute the tasks as I tell you to do them.
* Call: ` + t.String(),
	}, nil
}

type filesResult struct {
	Files []string `json:"files"`
}

func (s *Server) convertToBklOnFiles(p *taskcp.Project, t *taskcp.Task) error {
	result := filesResult{}

	if err := json.Unmarshal([]byte(t.Result), &result); err != nil {
		return fmt.Errorf("failed to parse file list: %w", err)
	}

	if len(result.Files) == 0 {
		return fmt.Errorf("no files provided")
	}

	helmFiles, err := s.convertHelmToPlain(result.Files)
	if err != nil {
		return fmt.Errorf("failed to convert Helm to plain YAML: %w", err)
	}
	if helmFiles != nil {
		result.Files = helmFiles
	} else {

		kustomizeFiles, err := s.convertKustomizeToPlain(result.Files)
		if err != nil {
			return fmt.Errorf("failed to convert Kustomize to plain YAML: %w", err)
		}
		if kustomizeFiles != nil {
			result.Files = kustomizeFiles
		}
	}

	commonPrefix := findCommonPrefix(result.Files)

	prepFiles := []string{}

	for _, file := range result.Files {
		prepFile := filepath.Join("prep", strings.TrimPrefix(file, commonPrefix))
		prepFiles = append(prepFiles, prepFile)

		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", file, err)
		}

		originalContent := string(content)
		p.AddLastTask().
			WithTitle("Convert to bkl").
			WithInstructions(`
* Convert the content in data["original_file"] to bkl patterns.
* You can read the pattern documentation with: mcp__bkl-mcp__get type="documentation" id="prep" source="k8s"
* You can look up other documentation and tests as needed: mcp__bkl-mcp__query keywords="..."
* Try to convert ALL lists to maps with names plus $encode: values, $encode: values:NAME or $encode: values:NAME:VALUE.
* Don't add comments
* If you run into a problem, try querying keywords="fixit" for previous solutions
* Return the converted bkl file contents in the results field of: {SUCCESS_PROMPT}`).
			WithData("original_file", originalContent).
			Then(func(t *taskcp.Task) error {
				return s.convertToBklOnPrepFile(p, prepFile, t)
			})

	}

	originalFileMap := make(map[string]string)
	for i, file := range result.Files {
		originalFileMap[prepFiles[i]] = file
	}

	p.AddLastTask().
		WithTitle("Determine bkl file structure").
		WithInstructions(`
* Read the documentation for file structure: mcp__bkl-mcp__get type="documentation" id="plan" source="k8s"
* The list of converted files is in data["prep_files"].
* Use the following command instead of bkli to examine file intersection: mcp__bkl-mcp__intersect selector="kind,metadata.name" files="prep/file1.yaml,prep/file2.yaml"
* Once you've determined the file structure, call: {SUCCESS_PROMPT}
* The result is a JSON encoding in the following format:
	{
		"files": {
			"prep/file1.yaml": "bkl/base.file1.yaml",
			"prep/file2.yaml": "bkl/base.file1.file2.yaml",
			"prep/file3.yaml": "bkl/base.file3.yaml"
		}
	}
* I'll figure out which files are in the base layer and which are in the derived layers.
* DO NOT create directories or files -- just use the tools to determine the file structure and tell me.
* selector="kind,metadata.name" is critical to tell intersect how to match documents
`).
		WithData("prep_files", prepFiles).
		Then(func(t *taskcp.Task) error {
			return s.convertToBklOnPlan(p, t, originalFileMap)
		})

	return nil
}

func (s *Server) convertToBklOnPrepFile(p *taskcp.Project, targetPath string, t *taskcp.Task) error {
	originalContent, ok := t.Data["original_file"].(string)
	if !ok {
		return fmt.Errorf("original_file not found in task data")
	}

	preppedContent := t.Result
	if preppedContent == "" {
		if prepped, ok := t.Data["prepped_file"].(string); ok {
			preppedContent = prepped
		}
	}

	if preppedContent == "" {
		return fmt.Errorf("no prepped content provided")
	}
	fsys := fstest.MapFS{
		"original.yaml": &fstest.MapFile{Data: []byte(originalContent)},
		"prepped.yaml":  &fstest.MapFile{Data: []byte(preppedContent)},
	}

	compareResult, err := bkl.Compare(fsys, "original.yaml", "prepped.yaml", "/", "/", nil, nil, nil)
	if err != nil {
		p.AddNextTask().
			WithTitle("Fix the prepped file").
			WithInstructions(`
* The comparison failed. Please fix the prepped file.
* The original file content is in data["original_file"].
* The prepped file content that failed is in data["prepped_file"].
* The error is in data["error"].
* Return the corrected bkl file contents in the result field of: {SUCCESS_PROMPT}`).
			WithData("original_file", originalContent).
			WithData("prepped_file", preppedContent).
			WithData("error", err.Error()).
			Then(func(t *taskcp.Task) error {
				return s.convertToBklOnPrepFile(p, targetPath, t)
			})

		return nil
	}

	if compareResult.Diff != "" && t.Result != "" {
		p.AddNextTask().
			WithTitle("Verify the conversion").
			WithInstructions(`
* The conversion resulted in different output.
* Verify the changes are correct.
* The original file content is in data["original_file"].
* The prepped file content is in data["prepped_file"].
* The diff is in data["diff"].
* If the changes are correct, respond with an empty string in the result field of: {SUCCESS_PROMPT}
* If you need to modify the conversion, provide the corrected bkl file contents in the result field.
* Changes in values after $encode conversion are usually bugs (other than list ordering). Don't convert back to a list -- figure out how to use $encode: values, $encode: values:NAME or $encode: values:NAME:VALUE.`).
			WithData("original_file", originalContent).
			WithData("prepped_file", preppedContent).
			WithData("diff", compareResult.Diff).
			Then(func(t *taskcp.Task) error {
				return s.convertToBklOnPrepFile(p, targetPath, t)
			})

		return nil
	}

	return s.writeConvertedFile(targetPath, preppedContent)
}

type planResult struct {
	Files map[string]string `json:"files"`
}

type fileInfo struct {
	prepFile   string
	targetFile string
	parent     string
}

func (s *Server) convertToBklOnPlan(p *taskcp.Project, t *taskcp.Task, originalFileMap map[string]string) error {
	result, err := s.parseFilePlan(t.Result)
	if err != nil {
		return err
	}

	files := s.buildFileInfoMap(result.Files)
	s.determineFileParents(files)

	if err := s.processFiles(files, result.Files); err != nil {
		return err
	}

	if err := s.createVerificationTasks(p, result.Files, originalFileMap); err != nil {
		return err
	}

	if err := s.createPolishTasks(p, result.Files, originalFileMap); err != nil {
		return err
	}

	s.createSummaryTask(p)

	return nil
}

func (s *Server) writeConvertedFile(targetPath string, content string) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", targetPath, err)
	}

	return nil
}

func (s *Server) verifyConversion(p *taskcp.Project, t *taskcp.Task, originalFile, targetFile string) error {
	if t.Result == "" {
		return nil
	}

	if err := s.writeConvertedFile(targetFile, t.Result); err != nil {
		return fmt.Errorf("failed to write updated file %s: %w", targetFile, err)
	}

	fsys := os.DirFS("/")
	compareResult, err := bkl.Compare(fsys, originalFile, targetFile, "/", "", nil, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to re-compare: %w", err)
	}

	if compareResult.Diff != "" {
		originalContent, err := os.ReadFile(originalFile)
		if err != nil {
			return fmt.Errorf("failed to read original file %s: %w", originalFile, err)
		}

		p.AddNextTask().
			WithTitle(fmt.Sprintf("Re-verify %s", filepath.Base(targetFile))).
			WithInstructions(`
* The bkl file still produces different output. 
* The diff is in data["diff"].
* Review and provide the updated bkl file content for %s in the result field of: {SUCCESS_PROMPT}
* Respond with an empty string if this is acceptable.`).
			WithData("original_content", string(originalContent)).
			WithData("target_content", t.Result).
			WithData("diff", compareResult.Diff).
			Then(func(t *taskcp.Task) error {
				return s.verifyConversion(p, t, originalFile, targetFile)
			})

		return nil
	}

	return nil
}

func findCommonPrefix(files []string) string {
	dir, _ := filepath.Split(files[0])
	commonParts := strings.Split(dir, string(filepath.Separator))

	for _, file := range files[1:] {
		dir, _ := filepath.Split(file)
		parts := strings.Split(dir, string(filepath.Separator))

		i := 0
		for i < len(commonParts) && i < len(parts) && commonParts[i] == parts[i] {
			i++
		}
		commonParts = commonParts[:i]
	}

	return strings.Join(commonParts, string(filepath.Separator))
}

func (s *Server) parseFilePlan(result string) (planResult, error) {
	plan := planResult{}

	if err := json.Unmarshal([]byte(result), &plan); err != nil {
		return plan, fmt.Errorf("failed to parse file plan: %w", err)
	}

	if len(plan.Files) == 0 {
		return plan, fmt.Errorf("no file mappings provided")
	}

	return plan, nil
}

func (s *Server) buildFileInfoMap(fileMap map[string]string) map[string]*fileInfo {
	files := make(map[string]*fileInfo)
	for prepFile, targetFile := range fileMap {
		files[targetFile] = &fileInfo{
			prepFile:   prepFile,
			targetFile: targetFile,
		}
	}
	return files
}

func (s *Server) determineFileParents(files map[string]*fileInfo) {
	for targetFile, info := range files {
		base := strings.TrimSuffix(targetFile, ".yaml")
		parts := strings.Split(base, ".")

		if len(parts) > 1 {
			parentBase := strings.Join(parts[:len(parts)-1], ".")
			parentFile := parentBase + ".yaml"
			info.parent = parentFile

			if _, exists := files[parentFile]; !exists {
				files[parentFile] = &fileInfo{
					targetFile: parentFile,
				}
			}
		}
	}
}

func (s *Server) processFiles(files map[string]*fileInfo, fileMap map[string]string) error {
	processed := make(map[string]bool)
	format := "yaml"

	var processFile func(targetFile string) error
	processFile = func(targetFile string) error {
		if processed[targetFile] {
			return nil
		}

		info := files[targetFile]
		if info == nil {
			return fmt.Errorf("file info not found for %s", targetFile)
		}

		if info.parent != "" {
			if err := processFile(info.parent); err != nil {
				return err
			}
		}

		if info.parent == "" {
			if err := s.processBaseLayer(info, files, fileMap, format); err != nil {
				return err
			}
		} else {
			if err := s.processDerivedLayer(info, format); err != nil {
				return err
			}
		}

		processed[targetFile] = true
		return nil
	}

	for targetFile := range files {
		if err := processFile(targetFile); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) processBaseLayer(info *fileInfo, files map[string]*fileInfo, fileMap map[string]string, format string) error {
	fsys := os.DirFS("/")
	var sourcesForBase []string

	if info.prepFile != "" {
		for prep, target := range fileMap {
			if target == info.targetFile {
				sourcesForBase = append(sourcesForBase, prep)
			}
		}
	} else {
		for _, childInfo := range files {
			if childInfo.parent == info.targetFile && childInfo.prepFile != "" {
				sourcesForBase = append(sourcesForBase, childInfo.prepFile)
			}
		}
	}

	if len(sourcesForBase) > 1 {
		output, err := bkl.Intersect(fsys, sourcesForBase, "/", "", []string{"kind"}, &format)
		if err != nil {
			return fmt.Errorf("failed to intersect files %v for base %s: %w", sourcesForBase, info.targetFile, err)
		}

		if err := s.writeConvertedFile(info.targetFile, string(output)); err != nil {
			return fmt.Errorf("failed to write base layer %s: %w", info.targetFile, err)
		}
	} else if len(sourcesForBase) == 1 {
		content, err := os.ReadFile(info.prepFile)
		if err != nil {
			return fmt.Errorf("failed to read source file %s: %w", info.prepFile, err)
		}

		if err := s.writeConvertedFile(info.targetFile, string(content)); err != nil {
			return fmt.Errorf("failed to write file %s: %w", info.targetFile, err)
		}
	}

	return nil
}

func (s *Server) processDerivedLayer(info *fileInfo, format string) error {
	fsys := os.DirFS("/")
	output, err := bkl.Diff(fsys, info.parent, info.prepFile, "/", "", []string{"kind"}, &format)
	if err != nil {
		return fmt.Errorf("failed to diff %s -> %s: %w", info.parent, info.prepFile, err)
	}

	if err := s.writeConvertedFile(info.targetFile, string(output)); err != nil {
		return fmt.Errorf("failed to write derived layer %s: %w", info.targetFile, err)
	}

	return nil
}

func (s *Server) createVerificationTasks(p *taskcp.Project, fileMap map[string]string, originalFileMap map[string]string) error {
	for prepFile, targetFile := range fileMap {
		originalFile := originalFileMap[prepFile]
		if originalFile == "" {
			continue
		}

		fsys := os.DirFS("/")
		compareResult, err := bkl.Compare(fsys, originalFile, targetFile, "/", "", nil, nil, nil)
		if err != nil {
			return fmt.Errorf("failed to compare %s: %w", originalFile, err)
		}

		if compareResult.Diff == "" {
			continue
		}

		originalContent, err := os.ReadFile(originalFile)
		if err != nil {
			return fmt.Errorf("failed to read original file %s: %w", originalFile, err)
		}
		targetContent, err := os.ReadFile(targetFile)
		if err != nil {
			return fmt.Errorf("failed to read target file %s: %w", targetFile, err)
		}

		p.AddLastTask().
			WithTitle("Verify bkl converion").
			WithInstructions(`
* Review the bkl conversion for %s.
* The diff between evaluating the original file and the bkl layered files is in data["diff"].
* If satisfied with the conversion, respond with an empty string in the result field of: {SUCCESS_PROMPT}
* If you want to modify the conversion, provide the updated bkl file content for %s in the result field.`).
			WithData("original_content", string(originalContent)).
			WithData("target_content", string(targetContent)).
			WithData("diff", compareResult.Diff).
			Then(func(t *taskcp.Task) error {
				return s.verifyConversion(p, t, originalFile, targetFile)
			})

		return nil
	}

	return nil
}

func (s *Server) createSummaryTask(p *taskcp.Project) {
	p.AddLastTask().
		WithTitle("Summarize conversion results").
		WithInstructions(`
* All file conversions have been completed.
* Provide a summary for the user.
* The task summary is in data["summary"].
* Call: {SUCCESS_PROMPT}
* Set the result field to your summary.`).
		WithData("summary", p.Summary().String()).
		Then(func(t *taskcp.Task) error {
			return nil
		})
}

func (s *Server) createPolishTasks(p *taskcp.Project, fileMap map[string]string, originalFileMap map[string]string) error {
	for _, targetFile := range fileMap {
		content, err := os.ReadFile(targetFile)
		if err != nil {
			return fmt.Errorf("failed to read file %s for polish: %w", targetFile, err)
		}

		p.AddLastTask().
			WithTitle(fmt.Sprintf("Polish %s", filepath.Base(targetFile))).
			WithInstructions(`
* Apply polish steps to improve the bkl file %s.
* Read the polish documentation: mcp__bkl-mcp__get type="documentation" id="polish" source="k8s"
* The current file content is in data["file_content"].
* If no polish is needed, respond with an empty string in the result field of: {SUCCESS_PROMPT}
* Otherwise, provide the polished bkl file content in the result field.
* Don't add comments`).
			WithData("file_content", string(content)).
			Then(func(t *taskcp.Task) error {
				return s.polishBklFile(p, t, targetFile, fileMap, originalFileMap)
			})

	}

	return nil
}

func (s *Server) polishBklFile(p *taskcp.Project, t *taskcp.Task, targetFile string, fileMap map[string]string, originalFileMap map[string]string) error {
	if t.Result == "" {
		return nil
	}

	if err := s.writeConvertedFile(targetFile, t.Result); err != nil {
		return fmt.Errorf("failed to write polished file %s: %w", targetFile, err)
	}

	return s.createSecondVerificationTask(p, targetFile, fileMap, originalFileMap)
}

func (s *Server) createSecondVerificationTask(p *taskcp.Project, targetFile string, fileMap map[string]string, originalFileMap map[string]string) error {
	var originalFile string

	for prep, target := range fileMap {
		if target == targetFile {
			originalFile = originalFileMap[prep]
			break
		}
	}

	if originalFile == "" {
		return nil
	}

	fsys := os.DirFS("/")
	compareResult, err := bkl.Compare(fsys, originalFile, targetFile, "/", "", nil, nil, nil)
	if err != nil {
		compareErr := err

		originalContent, err := os.ReadFile(originalFile)
		if err != nil {
			return fmt.Errorf("failed to read original file %s: %w", originalFile, err)
		}
		targetContent, err := os.ReadFile(targetFile)
		if err != nil {
			return fmt.Errorf("failed to read target file %s: %w", targetFile, err)
		}

		p.AddNextTask().
			WithTitle("Fix comparison error").
			WithInstructions(`
* The comparison between the original file and bkl conversion failed.
* Provide the updated bkl file content in the result field of: {SUCCESS_PROMPT}`).
			WithData("error", compareErr.Error()).
			WithData("target_content", string(targetContent)).
			WithData("original_content", string(originalContent)).
			Then(func(t *taskcp.Task) error {
				return s.verifyConversion(p, t, originalFile, targetFile)
			})

		return nil
	}

	if compareResult.Diff == "" {
		return nil
	}

	originalContent, err := os.ReadFile(originalFile)
	if err != nil {
		return fmt.Errorf("failed to read original file %s: %w", originalFile, err)
	}
	targetContent, err := os.ReadFile(targetFile)
	if err != nil {
		return fmt.Errorf("failed to read target file %s: %w", targetFile, err)
	}

	p.AddNextTask().
		WithTitle("Re-verify after polish").
		WithInstructions(`
* Review the bkl conversion after polish has been applied.
* The diff between evaluating the original file and the polished bkl layered files is in data["diff"].
* If satisfied with the polish changes, respond with an empty string in the result field of: {SUCCESS_PROMPT}
* If you want to modify the polish changes, provide the updated bkl file content for %s in the result field.`).
		WithData("original_content", string(originalContent)).
		WithData("target_content", string(targetContent)).
		WithData("diff", compareResult.Diff).
		Then(func(t *taskcp.Task) error {
			return s.verifyConversion(p, t, originalFile, targetFile)
		})

	return nil
}
