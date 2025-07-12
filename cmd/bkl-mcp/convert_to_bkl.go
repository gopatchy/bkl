package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing/fstest"

	"github.com/gopatchy/bkl"
	"github.com/gopatchy/taskcp"
)

func (s *Server) convertToBklHandler(ctx context.Context, args struct{}) (*promptResponse, error) {
	p := s.taskService.AddProject()

	task := p.InsertTaskBefore(
		"",
		"Find configuration files",
		`Find all the configuration files you need to convert. Then call:

{SUCCESS_PROMPT}

where result contains a JSON encoding in the following format:

{
	"files": [
		"path/to/file1.yaml",
		"path/to/file2.yaml"
	]
}
`,
		func(p *taskcp.Project, task *taskcp.Task) error {
			return s.convertToBklOnFiles(p, task)
		},
	)

	return &promptResponse{
		Prompt: `# Converting Kubernetes configuration files to bkl format

I'll walk you through this process step by step. Follow EXACTLY these steps -- do not attempt to do the conversion yourself or follow any steps that are not EXACTLY what I tell you to do. DO NOT invent a TODO list -- just execute the tasks as I tell you to do them.

First execute:

` + task.String(),
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
		task := p.InsertTaskBefore(
			"",
			"Convert to bkl",
			`Convert the content in data["original_file"] to bkl patterns.

You can read the pattern documentation with:

mcp__bkl-mcp__get type="documentation" id="prep" source="k8s"

You can look up other documentation and tests as needed:

mcp__bkl-mcp__query keywords="..."

Hints:
* Try to convert ALL lists to maps with names plus $encode: values, $encode: values:NAME or $encode: values:NAME:VALUE.

Return the converted bkl file contents in the results field of:

{SUCCESS_PROMPT}
`,
			func(p *taskcp.Project, t *taskcp.Task) error {
				return s.convertToBklOnPrepFile(p, prepFile, t)
			},
		)

		task.Data["original_file"] = originalContent
	}

	originalFileMap := make(map[string]string)
	for i, file := range result.Files {
		originalFileMap[prepFiles[i]] = file
	}

	task := p.InsertTaskBefore(
		"",
		"Determine bkl file structure",
		`Read the documentation for file structure:

mcp__bkl-mcp__get type="documentation" id="plan" source="k8s"

The list of converted files is in data["prep_files"].

Use the following command instead of bkli to examine file intersection:

mcp__bkl-mcp__intersect selector="kind" files="prep/file1.yaml,prep/file2.yaml"

Once you've determined the file structure, call:

{SUCCESS_PROMPT}

where the result is a JSON encoding in the following format:

{
	"files":
		"prep/file1.yaml": "bkl/base.file1.yaml",
		"prep/file2.yaml": "bkl/base.file1.file2.yaml",
		"prep/file3.yaml": "bkl/base.file3.yaml"
	]
}

I'll figure out which files are in the base layer and which are in the derived layers.

DO NOT create directories or files -- just use the tools to determine the file structure and tell me.
`,
		func(p *taskcp.Project, t *taskcp.Task) error {
			return s.convertToBklOnPlan(p, t, originalFileMap)
		},
	)

	task.Data["prep_files"] = prepFiles

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

	compareResult, err := bkl.Compare(fsys, "original.yaml", "prepped.yaml", "/", "/", nil, nil, "")
	if err != nil {
		fixTask := p.InsertTaskBefore(
			t.NextTaskID,
			"Fix the prepped file",
			`The comparison failed. Please fix the prepped file.

The original file content is in data["original_file"].
The prepped file content that failed is in data["prepped_file"].
The error is in data["error"].

Return the corrected bkl file contents in the result field of:

{SUCCESS_PROMPT}`,
			func(p *taskcp.Project, t *taskcp.Task) error {
				return s.convertToBklOnPrepFile(p, targetPath, t)
			},
		)

		fixTask.Data["original_file"] = originalContent
		fixTask.Data["prepped_file"] = preppedContent
		fixTask.Data["error"] = err.Error()

		return nil
	}

	if compareResult.Diff != "" && t.Result != "" {
		verifyTask := p.InsertTaskBefore(
			t.NextTaskID,
			"Verify the conversion",
			`The conversion resulted in different output. Please verify the changes are correct.

The original file content is in data["original_file"].
The prepped file content is in data["prepped_file"].
The diff is in data["diff"].

If the changes are correct, respond with an empty string in the result field of:
{SUCCESS_PROMPT}

If you need to modify the conversion, provide the corrected bkl file contents in the result field.`,
			func(p *taskcp.Project, t *taskcp.Task) error {
				return s.convertToBklOnPrepFile(p, targetPath, t)
			},
		)

		verifyTask.Data["original_file"] = originalContent
		verifyTask.Data["prepped_file"] = preppedContent
		verifyTask.Data["diff"] = compareResult.Diff

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
	compareResult, err := bkl.Compare(fsys, originalFile, targetFile, "/", "/", nil, nil, "")
	if err != nil {
		return fmt.Errorf("failed to re-compare: %w", err)
	}

	if compareResult.Diff != "" {
		originalContent, err := os.ReadFile(originalFile)
		if err != nil {
			return fmt.Errorf("failed to read original file %s: %w", originalFile, err)
		}

		retryTask := p.InsertTaskBefore(
			t.NextTaskID,
			fmt.Sprintf("Re-verify %s", filepath.Base(targetFile)),
			fmt.Sprintf(`The bkl file still produces different output. 

The diff is in data["diff"].

Review and provide the updated bkl file content for %s in the result field of {SUCCESS_PROMPT}, or respond with an empty string if this is acceptable.`, targetFile),
			func(p *taskcp.Project, t *taskcp.Task) error {
				return s.verifyConversion(p, t, originalFile, targetFile)
			},
		)

		retryTask.Data["original_content"] = string(originalContent)
		retryTask.Data["target_content"] = t.Result
		retryTask.Data["diff"] = compareResult.Diff
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
		output, err := bkl.Intersect(fsys, sourcesForBase, "/", "", "kind", &format)
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
	output, err := bkl.Diff(fsys, info.parent, info.prepFile, "/", "", "kind", &format)
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
		compareResult, err := bkl.Compare(fsys, originalFile, targetFile, "/", "", nil, nil, "")
		if err != nil {
			return fmt.Errorf("failed to compare %s: %w", originalFile, err)
		}

		if compareResult.Diff == "" {
			continue
		}

		task := p.InsertTaskBefore(
			"",
			fmt.Sprintf("Verify %s", filepath.Base(originalFile)),
			fmt.Sprintf(`Review the bkl conversion for %s.

The diff between evaluating the original file and the bkl layered files is in data["diff"].

If satisfied with the conversion, respond with an empty string in the result field of:
{SUCCESS_PROMPT}

If you want to modify the conversion, provide the updated bkl file content for %s in the result field.`, originalFile, targetFile),
			func(p *taskcp.Project, t *taskcp.Task) error {
				return s.verifyConversion(p, t, originalFile, targetFile)
			},
		)

		originalContent, err := os.ReadFile(originalFile)
		if err != nil {
			return fmt.Errorf("failed to read original file %s: %w", originalFile, err)
		}
		targetContent, err := os.ReadFile(targetFile)
		if err != nil {
			return fmt.Errorf("failed to read target file %s: %w", targetFile, err)
		}

		task.Data["original_content"] = string(originalContent)
		task.Data["target_content"] = string(targetContent)
		task.Data["diff"] = compareResult.Diff
	}

	return nil
}

func (s *Server) createSummaryTask(p *taskcp.Project) {
	summaryTask := p.InsertTaskBefore(
		"",
		"Summarize conversion results",
		`All file conversions have been completed. Please provide a summary for the user.

The task summary is in data["summary"].

Call {SUCCESS_PROMPT} with your summary in the result field.`,
		func(p *taskcp.Project, t *taskcp.Task) error {
			return nil
		},
	)

	summaryTask.Data["summary"] = p.Summary().String()
}
