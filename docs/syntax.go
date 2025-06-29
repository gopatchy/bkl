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

func highlightYAML(text string, offset int) []insertion {
	h := &syntaxHighlighter{
		text:   text,
		offset: offset,
	}

	state := "lineStart"
	tokenStart := 0
	quoteChar := byte(0)
	escapeNext := false

	for pos := 0; pos < len(text); pos++ {
		ch := text[pos]

		switch state {
		case "lineStart":
			if ch == ' ' || ch == '\t' {
				// Skip indentation
			} else if ch == '#' {
				tokenStart = pos
				state = "comment"
			} else if ch == '-' {
				state = "maybeDash"
			} else if ch == '\n' {
				// Stay in lineStart
			} else if ch == '"' || ch == '\'' {
				tokenStart = pos
				quoteChar = ch
				state = "string"
			} else {
				tokenStart = pos
				state = "keyOrScalar"
			}

		case "maybeDash":
			if ch == ' ' || ch == '\n' {
				state = "listValue"
			} else {
				// It wasn't a list marker, treat dash as start of key
				tokenStart = pos - 1
				state = "keyOrScalar"
			}

		case "listValue":
			if ch == ' ' || ch == '\t' {
				// Skip spaces after dash
			} else if ch == '\n' {
				state = "lineStart"
			} else if ch == '#' {
				tokenStart = pos
				state = "comment"
			} else if ch == '"' || ch == '\'' {
				tokenStart = pos
				quoteChar = ch
				state = "string"
			} else {
				tokenStart = pos
				state = "keyOrScalar"
			}

		case "keyOrScalar":
			if ch == ':' {
				state = "colonFound"
			} else if ch == '\n' {
				// It was just a scalar value
				value := strings.TrimSpace(text[tokenStart:pos])
				if value == "true" || value == "false" {
					h.addToken("bool", tokenStart, tokenStart+len(value))
				} else if isNumber(value) {
					h.addToken("number", tokenStart, tokenStart+len(value))
				} else if len(value) > 0 {
					h.addToken("string", tokenStart, tokenStart+len(value))
				}
				state = "lineStart"
			}

		case "colonFound":
			if ch == ' ' || ch == '\n' || ch == '\t' {
				// It's a key
				h.addToken("key", tokenStart, pos-1)
				state = "afterColon"
				if ch == '\n' {
					state = "lineStart"
				}
			} else {
				// Colon wasn't a key separator, continue as scalar
				state = "scalar"
			}

		case "afterColon":
			if ch == ' ' || ch == '\t' {
				// Skip spaces after colon
			} else if ch == '\n' {
				state = "lineStart"
			} else if ch == '#' {
				tokenStart = pos
				state = "comment"
			} else if ch == '"' || ch == '\'' {
				tokenStart = pos
				quoteChar = ch
				state = "string"
			} else {
				tokenStart = pos
				state = "scalar"
			}

		case "scalar":
			if ch == '\n' || ch == '#' {
				value := strings.TrimSpace(text[tokenStart:pos])
				if value == "true" || value == "false" {
					h.addToken("bool", tokenStart, tokenStart+len(value))
				} else if isNumber(value) {
					h.addToken("number", tokenStart, tokenStart+len(value))
				} else if len(value) > 0 {
					h.addToken("string", tokenStart, tokenStart+len(value))
				}

				if ch == '\n' {
					state = "lineStart"
				} else {
					tokenStart = pos
					state = "comment"
				}
			}

		case "string":
			if escapeNext {
				escapeNext = false
			} else if ch == '\\' && quoteChar == '"' {
				escapeNext = true
			} else if ch == quoteChar {
				h.addToken("string", tokenStart, pos+1)
				state = "afterString"
			} else if ch == '\n' && quoteChar == '\'' {
				// Single-quoted strings don't support multi-line in our simplified parser
				h.addToken("string", tokenStart, pos)
				state = "lineStart"
			}

		case "afterString":
			if ch == '\n' {
				state = "lineStart"
			} else if ch == '#' {
				tokenStart = pos
				state = "comment"
			} else if ch != ' ' && ch != '\t' {
				// More content after string
				state = "scalar"
			}

		case "comment":
			if ch == '\n' {
				h.addToken("comment", tokenStart, pos)
				state = "lineStart"
			}
		}
	}

	// Handle any remaining tokens at end of text
	switch state {
	case "comment":
		h.addToken("comment", tokenStart, len(text))
	case "string":
		h.addToken("string", tokenStart, len(text))
	case "scalar", "keyOrScalar":
		value := strings.TrimSpace(text[tokenStart:])
		if value == "true" || value == "false" {
			h.addToken("bool", tokenStart, tokenStart+len(value))
		} else if isNumber(value) {
			h.addToken("number", tokenStart, tokenStart+len(value))
		} else if len(value) > 0 {
			h.addToken("string", tokenStart, tokenStart+len(value))
		}
	}

	return h.insertions
}

func highlightTOML(text string, offset int) []insertion {
	h := &syntaxHighlighter{
		text:   text,
		offset: offset,
	}

	state := "lineStart"
	tokenStart := 0
	quoteChar := byte(0)
	escapeNext := false
	bracketDepth := 0

	for pos := 0; pos < len(text); pos++ {
		ch := text[pos]

		switch state {
		case "lineStart":
			if ch == ' ' || ch == '\t' {
				// Skip indentation
			} else if ch == '#' {
				tokenStart = pos
				state = "comment"
			} else if ch == '[' {
				tokenStart = pos
				bracketDepth = 1
				state = "section"
			} else if ch == '\n' {
				// Stay in lineStart
			} else if ch == '"' || ch == '\'' {
				tokenStart = pos
				quoteChar = ch
				state = "keyString"
			} else {
				tokenStart = pos
				state = "key"
			}

		case "section":
			if ch == '[' {
				bracketDepth++
			} else if ch == ']' {
				bracketDepth--
				if bracketDepth == 0 {
					h.addToken("key", tokenStart, pos+1)
					state = "afterSection"
				}
			} else if ch == '\n' {
				// Invalid section header
				state = "lineStart"
			}

		case "afterSection":
			if ch == '\n' {
				state = "lineStart"
			} else if ch == '#' {
				tokenStart = pos
				state = "comment"
			} else if ch != ' ' && ch != '\t' {
				// Invalid content after section
				state = "lineStart"
			}

		case "key":
			if ch == '=' {
				h.addToken("key", tokenStart, pos)
				state = "afterEquals"
			} else if ch == '\n' {
				// Key without value
				state = "lineStart"
			} else if ch == ' ' || ch == '\t' {
				// End of key, expect equals
				h.addToken("key", tokenStart, pos)
				state = "beforeEquals"
			}

		case "keyString":
			if escapeNext {
				escapeNext = false
			} else if ch == '\\' {
				escapeNext = true
			} else if ch == quoteChar {
				state = "afterKeyString"
			} else if ch == '\n' && quoteChar == '\'' {
				// Single-quoted strings don't support multi-line
				h.addToken("key", tokenStart, pos)
				state = "lineStart"
			}

		case "afterKeyString":
			if ch == ' ' || ch == '\t' {
				// Skip whitespace
			} else if ch == '=' {
				h.addToken("key", tokenStart, pos)
				state = "afterEquals"
			} else if ch == '\n' {
				h.addToken("key", tokenStart, pos)
				state = "lineStart"
			}

		case "beforeEquals":
			if ch == ' ' || ch == '\t' {
				// Skip whitespace
			} else if ch == '=' {
				state = "afterEquals"
			} else if ch == '\n' {
				state = "lineStart"
			}

		case "afterEquals":
			if ch == ' ' || ch == '\t' {
				// Skip whitespace
			} else if ch == '\n' {
				state = "lineStart"
			} else if ch == '#' {
				tokenStart = pos
				state = "comment"
			} else if ch == '"' || ch == '\'' {
				tokenStart = pos
				quoteChar = ch
				state = "valueString"
			} else if ch == '[' {
				tokenStart = pos
				state = "array"
			} else if ch == '{' {
				tokenStart = pos
				state = "inlineTable"
			} else if (ch >= '0' && ch <= '9') || ch == '-' || ch == '+' {
				tokenStart = pos
				state = "number"
			} else if ch == 't' || ch == 'f' {
				tokenStart = pos
				state = "boolean"
			} else {
				tokenStart = pos
				state = "bareString"
			}

		case "valueString":
			if escapeNext {
				escapeNext = false
			} else if ch == '\\' {
				escapeNext = true
			} else if ch == quoteChar {
				h.addToken("string", tokenStart, pos+1)
				state = "afterValue"
			} else if ch == '\n' && quoteChar == '\'' {
				// Single-quoted strings don't support multi-line in our simplified parser
				h.addToken("string", tokenStart, pos)
				state = "lineStart"
			}

		case "bareString":
			if ch == '\n' || ch == '#' || ch == ',' || ch == ']' || ch == '}' {
				h.addToken("string", tokenStart, pos)
				if ch == '\n' {
					state = "lineStart"
				} else if ch == '#' {
					tokenStart = pos
					state = "comment"
				} else if ch == ',' {
					state = "afterComma"
				} else {
					state = "inValue"
				}
			}

		case "number":
			if ch == '\n' || ch == '#' || ch == ',' || ch == ']' || ch == '}' || ch == ' ' || ch == '\t' {
				h.addToken("number", tokenStart, pos)
				if ch == '\n' {
					state = "lineStart"
				} else if ch == '#' {
					tokenStart = pos
					state = "comment"
				} else if ch == ',' {
					state = "afterComma"
				} else if ch == ']' || ch == '}' {
					state = "inValue"
				} else {
					state = "afterValue"
				}
			} else if (ch >= '0' && ch <= '9') || ch == '.' || ch == 'e' || ch == 'E' || ch == '+' || ch == '-' || ch == '_' || ch == ':' || ch == 'T' || ch == 'Z' {
				// Continue number (including dates/times)
			} else {
				// End of number
				h.addToken("number", tokenStart, pos)
				state = "afterValue"
			}

		case "boolean":
			if ch == '\n' || ch == '#' || ch == ',' || ch == ']' || ch == '}' || ch == ' ' || ch == '\t' {
				value := text[tokenStart:pos]
				if value == "true" || value == "false" {
					h.addToken("bool", tokenStart, pos)
				}
				if ch == '\n' {
					state = "lineStart"
				} else if ch == '#' {
					tokenStart = pos
					state = "comment"
				} else if ch == ',' {
					state = "afterComma"
				} else if ch == ']' || ch == '}' {
					state = "inValue"
				} else {
					state = "afterValue"
				}
			} else if ch >= 'a' && ch <= 'z' {
				// Continue boolean
			} else {
				// Not a boolean
				state = "bareString"
			}

		case "array":
			if ch == ']' {
				state = "afterValue"
			} else if ch == ' ' || ch == '\t' || ch == '\n' {
				// Skip whitespace
			} else if ch == '#' {
				tokenStart = pos
				state = "comment"
			} else if ch == '"' || ch == '\'' {
				tokenStart = pos
				quoteChar = ch
				state = "valueString"
			} else if ch == '[' {
				// Nested array
			} else if (ch >= '0' && ch <= '9') || ch == '-' || ch == '+' {
				tokenStart = pos
				state = "number"
			} else if ch == 't' || ch == 'f' {
				tokenStart = pos
				state = "boolean"
			} else {
				tokenStart = pos
				state = "bareString"
			}

		case "inlineTable":
			// Simplified inline table handling
			if ch == '}' {
				state = "afterValue"
			}

		case "afterComma":
			if ch == ' ' || ch == '\t' || ch == '\n' {
				// Skip whitespace
			} else if ch == '"' || ch == '\'' {
				tokenStart = pos
				quoteChar = ch
				state = "valueString"
			} else if (ch >= '0' && ch <= '9') || ch == '-' || ch == '+' {
				tokenStart = pos
				state = "number"
			} else if ch == 't' || ch == 'f' {
				tokenStart = pos
				state = "boolean"
			} else {
				tokenStart = pos
				state = "bareString"
			}

		case "afterValue":
			if ch == '\n' {
				state = "lineStart"
			} else if ch == '#' {
				tokenStart = pos
				state = "comment"
			} else if ch == ',' {
				state = "afterComma"
			} else if ch == ']' || ch == '}' {
				state = "afterValue"
			}

		case "inValue":
			// Generic state for when we're inside a complex value
			if ch == '\n' {
				state = "lineStart"
			} else if ch == ']' || ch == '}' {
				state = "afterValue"
			} else if ch == ',' {
				state = "afterComma"
			}

		case "comment":
			if ch == '\n' {
				h.addToken("comment", tokenStart, pos)
				state = "lineStart"
			}
		}
	}

	// Handle any remaining tokens at end of text
	switch state {
	case "comment":
		h.addToken("comment", tokenStart, len(text))
	case "valueString", "keyString":
		h.addToken("string", tokenStart, len(text))
	case "bareString":
		h.addToken("string", tokenStart, len(text))
	case "number":
		h.addToken("number", tokenStart, len(text))
	case "boolean":
		value := text[tokenStart:]
		if value == "true" || value == "false" {
			h.addToken("bool", tokenStart, len(text))
		}
	case "key":
		h.addToken("key", tokenStart, len(text))
	case "section":
		h.addToken("key", tokenStart, len(text))
	}

	return h.insertions
}

func highlightJSON(text string, offset int) []insertion {
	h := &syntaxHighlighter{
		text:   text,
		offset: offset,
	}

	state := "value"
	tokenStart := 0
	escapeNext := false
	contextStack := []string{}

	for pos := 0; pos < len(text); pos++ {
		ch := text[pos]

		switch state {
		case "value":
			if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
				// Skip whitespace
			} else if ch == '"' {
				tokenStart = pos
				state = "string"
			} else if ch == '{' {
				contextStack = append(contextStack, "object")
				state = "objectStart"
			} else if ch == '[' {
				contextStack = append(contextStack, "array")
				state = "value"
			} else if ch == 't' || ch == 'f' || ch == 'n' {
				tokenStart = pos
				state = "keyword"
			} else if ch == '-' || (ch >= '0' && ch <= '9') {
				tokenStart = pos
				state = "number"
			} else if ch == '}' || ch == ']' {
				if len(contextStack) > 0 {
					contextStack = contextStack[:len(contextStack)-1]
				}
				if len(contextStack) == 0 {
					state = "done"
				} else {
					state = "afterValue"
				}
			}

		case "objectStart":
			if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
				// Skip whitespace
			} else if ch == '"' {
				tokenStart = pos
				state = "objectKey"
			} else if ch == '}' {
				if len(contextStack) > 0 {
					contextStack = contextStack[:len(contextStack)-1]
				}
				if len(contextStack) == 0 {
					state = "done"
				} else {
					state = "afterValue"
				}
			}

		case "objectKey":
			if escapeNext {
				escapeNext = false
			} else if ch == '\\' {
				escapeNext = true
			} else if ch == '"' {
				h.addToken("key", tokenStart, pos+1)
				state = "afterKey"
			}

		case "afterKey":
			if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
				// Skip whitespace
			} else if ch == ':' {
				state = "value"
			}

		case "string":
			if escapeNext {
				escapeNext = false
			} else if ch == '\\' {
				escapeNext = true
			} else if ch == '"' {
				h.addToken("string", tokenStart, pos+1)
				state = "afterValue"
			}

		case "number":
			if ch == ',' || ch == '}' || ch == ']' || ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
				h.addToken("number", tokenStart, pos)
				if ch == ',' {
					context := ""
					if len(contextStack) > 0 {
						context = contextStack[len(contextStack)-1]
					}
					if context == "object" {
						state = "expectKey"
					} else {
						state = "value"
					}
				} else if ch == '}' || ch == ']' {
					if len(contextStack) > 0 {
						contextStack = contextStack[:len(contextStack)-1]
					}
					if len(contextStack) == 0 {
						state = "done"
					} else {
						state = "afterValue"
					}
				} else {
					state = "afterValue"
				}
			} else if (ch >= '0' && ch <= '9') || ch == '.' || ch == 'e' || ch == 'E' || ch == '+' || ch == '-' {
				// Continue number
			} else {
				// Invalid number
				state = "error"
			}

		case "keyword":
			if ch == ',' || ch == '}' || ch == ']' || ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
				keyword := text[tokenStart:pos]
				if keyword == "true" || keyword == "false" {
					h.addToken("bool", tokenStart, pos)
				} else if keyword == "null" {
					h.addToken("keyword", tokenStart, pos)
				}
				if ch == ',' {
					context := ""
					if len(contextStack) > 0 {
						context = contextStack[len(contextStack)-1]
					}
					if context == "object" {
						state = "expectKey"
					} else {
						state = "value"
					}
				} else if ch == '}' || ch == ']' {
					if len(contextStack) > 0 {
						contextStack = contextStack[:len(contextStack)-1]
					}
					if len(contextStack) == 0 {
						state = "done"
					} else {
						state = "afterValue"
					}
				} else {
					state = "afterValue"
				}
			} else if ch >= 'a' && ch <= 'z' {
				// Continue keyword
			} else {
				// Invalid keyword
				state = "error"
			}

		case "afterValue":
			if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
				// Skip whitespace
			} else if ch == ',' {
				context := ""
				if len(contextStack) > 0 {
					context = contextStack[len(contextStack)-1]
				}
				if context == "object" {
					state = "expectKey"
				} else {
					state = "value"
				}
			} else if ch == '}' || ch == ']' {
				if len(contextStack) > 0 {
					contextStack = contextStack[:len(contextStack)-1]
				}
				if len(contextStack) == 0 {
					state = "done"
				} else {
					state = "afterValue"
				}
			}

		case "expectKey":
			if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
				// Skip whitespace
			} else if ch == '"' {
				tokenStart = pos
				state = "objectKey"
			} else if ch == '}' {
				if len(contextStack) > 0 {
					contextStack = contextStack[:len(contextStack)-1]
				}
				if len(contextStack) == 0 {
					state = "done"
				} else {
					state = "afterValue"
				}
			}

		case "error", "done":
			// Stop processing
			return h.insertions
		}
	}

	// Handle any remaining tokens at end of text
	switch state {
	case "string":
		h.addToken("string", tokenStart, len(text))
	case "objectKey":
		h.addToken("key", tokenStart, len(text))
	case "number":
		h.addToken("number", tokenStart, len(text))
	case "keyword":
		keyword := text[tokenStart:]
		if keyword == "true" || keyword == "false" {
			h.addToken("bool", tokenStart, len(text))
		} else if keyword == "null" {
			h.addToken("keyword", tokenStart, len(text))
		}
	}

	return h.insertions
}

func highlightShell(text string, offset int) []insertion {
	// TODO: Implement shell syntax highlighting
	return []insertion{}
}

func highlightDiff(text string, offset int) []insertion {
	// TODO: Implement diff syntax highlighting
	return []insertion{}
}

func highlightDockerfile(text string, offset int) []insertion {
	// TODO: Implement Dockerfile syntax highlighting
	return []insertion{}
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
