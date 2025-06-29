package main

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
)

type insertion struct {
	pos      int
	text     string
	priority int
}

type syntaxHighlighter struct {
	text       string
	offset     int
	insertions []insertion
}

func (h *syntaxHighlighter) addToken(tokenType string, start, end int) {
	h.insertions = append(h.insertions, insertion{
		pos:      h.offset + start,
		text:     fmt.Sprintf("<%s>", tokenType),
		priority: 1,
	})
	h.insertions = append(h.insertions, insertion{
		pos:      h.offset + end,
		text:     fmt.Sprintf("</%s>", tokenType),
		priority: 1,
	})
}

func isNumber(s string) bool {
	match, _ := regexp.MatchString(`^-?\d+(\.\d+)?$`, s)
	return match
}

func getLineNumber(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		return 0
	}
}

func applySyntaxHighlighting(code string, languages [][]interface{}, highlights []string) string {
	var insertions []insertion

	// First add highlight insertions with special priority
	// Sort highlights from longest to shortest to avoid overlapping replacements
	sortedHighlights := make([]string, len(highlights))
	copy(sortedHighlights, highlights)
	slices.SortFunc(sortedHighlights, func(a, b string) int {
		return len(b) - len(a) // Sort by length descending
	})

	for _, highlight := range sortedHighlights {
		index := 0
		for {
			pos := strings.Index(code[index:], highlight)
			if pos == -1 {
				break
			}
			actualPos := index + pos
			// Opening <highlight>: priority 0 (inserted last, appears first/outermost)
			insertions = append(insertions, insertion{
				pos:      actualPos,
				text:     "<highlight>",
				priority: 0,
			})
			// Closing </highlight>: priority 2 (inserted first, appears last/outermost)
			insertions = append(insertions, insertion{
				pos:      actualPos + len(highlight),
				text:     "</highlight>",
				priority: 2,
			})
			index = actualPos + len(highlight)
		}
	}

	// Then add syntax highlighting insertions
	if len(languages) == 0 {
		return applyInsertions(code, insertions)
	}

	pos := 0
	lineNum := 0
	nextLangIndex := 1
	chunkStart := 0

	for pos < len(code) {
		if nextLangIndex < len(languages) && getLineNumber(languages[nextLangIndex][0]) == lineNum {
			// Process the current chunk before switching languages
			chunkText := code[chunkStart:pos]
			chunkLang := languages[nextLangIndex-1][1].(string)
			insertions = append(insertions, highlightChunk(chunkText, chunkLang, chunkStart)...)

			chunkStart = pos
			nextLangIndex++
		}

		nextNewline := strings.Index(code[pos:], "\n")
		if nextNewline == -1 {
			break
		}

		pos += nextNewline + 1
		lineNum++
	}

	// Process the final chunk
	if chunkStart < len(code) {
		chunkText := code[chunkStart:]
		chunkLang := languages[nextLangIndex-1][1].(string)
		insertions = append(insertions, highlightChunk(chunkText, chunkLang, chunkStart)...)
	}

	// Apply insertions
	return applyInsertions(code, insertions)
}

func highlightChunk(text, language string, offset int) []insertion {
	switch language {
	case "yaml":
		return highlightYAML(text, offset)
	case "toml":
		return highlightTOML(text, offset)
	case "json":
		return highlightJSON(text, offset)
	case "shell":
		return highlightShell(text, offset)
	case "diff":
		return highlightDiff(text, offset)
	case "dockerfile":
		return highlightDockerfile(text, offset)
	default:
		return []insertion{}
	}
}

func applyInsertions(text string, insertions []insertion) string {
	// Sort by position (descending), then by priority
	sort.Slice(insertions, func(i, j int) bool {
		if insertions[i].pos == insertions[j].pos {
			// At same position, sort by priority
			// Since we insert right-to-left, higher priority number gets inserted first
			return insertions[i].priority > insertions[j].priority
		}
		return insertions[i].pos > insertions[j].pos
	})

	result := text
	for _, ins := range insertions {
		result = result[:ins.pos] + ins.text + result[ins.pos:]
	}

	return result
}
