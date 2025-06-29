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

// YAML state machine states
type yamlState int

const (
	yamlLineStart yamlState = iota
	yamlMaybeDash
	yamlListValue
	yamlKeyOrScalar
	yamlColonFound
	yamlAfterColon
	yamlScalar
	yamlString
	yamlAfterString
	yamlComment
)

// TOML state machine states
type tomlState int

const (
	tomlLineStart tomlState = iota
	tomlSection
	tomlAfterSection
	tomlKey
	tomlKeyString
	tomlAfterKeyString
	tomlBeforeEquals
	tomlAfterEquals
	tomlValueString
	tomlBareString
	tomlNumber
	tomlBoolean
	tomlArray
	tomlInlineTable
	tomlAfterComma
	tomlAfterValue
	tomlInValue
	tomlComment
)

// JSON state machine states
type jsonState int

const (
	jsonValue jsonState = iota
	jsonObjectStart
	jsonObjectKey
	jsonAfterKey
	jsonString
	jsonNumber
	jsonKeyword
	jsonAfterValue
	jsonExpectKey
	jsonError
	jsonDone
)

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

	state := yamlLineStart
	tokenStart := 0
	quoteChar := byte(0)
	escapeNext := false

	for pos := 0; pos < len(text); pos++ {
		ch := text[pos]

		switch state {
		case yamlLineStart:
			if ch == ' ' || ch == '\t' {
				// Skip indentation
			} else if ch == '#' {
				tokenStart = pos
				state = yamlComment
			} else if ch == '-' {
				state = yamlMaybeDash
			} else if ch == '\n' {
				// Stay in lineStart
			} else if ch == '"' || ch == '\'' {
				tokenStart = pos
				quoteChar = ch
				state = yamlString
			} else {
				tokenStart = pos
				state = yamlKeyOrScalar
			}

		case yamlMaybeDash:
			if ch == ' ' || ch == '\n' {
				state = yamlListValue
			} else {
				// It wasn't a list marker, treat dash as start of key
				tokenStart = pos - 1
				state = yamlKeyOrScalar
			}

		case yamlListValue:
			if ch == ' ' || ch == '\t' {
				// Skip spaces after dash
			} else if ch == '\n' {
				state = yamlLineStart
			} else if ch == '#' {
				tokenStart = pos
				state = yamlComment
			} else if ch == '"' || ch == '\'' {
				tokenStart = pos
				quoteChar = ch
				state = yamlString
			} else {
				tokenStart = pos
				state = yamlKeyOrScalar
			}

		case yamlKeyOrScalar:
			if ch == ':' {
				state = yamlColonFound
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
				state = yamlLineStart
			}

		case yamlColonFound:
			if ch == ' ' || ch == '\n' || ch == '\t' {
				// It's a key
				h.addToken("key", tokenStart, pos-1)
				state = yamlAfterColon
				if ch == '\n' {
					state = yamlLineStart
				}
			} else {
				// Colon wasn't a key separator, continue as scalar
				state = yamlScalar
			}

		case yamlAfterColon:
			if ch == ' ' || ch == '\t' {
				// Skip spaces after colon
			} else if ch == '\n' {
				state = yamlLineStart
			} else if ch == '#' {
				tokenStart = pos
				state = yamlComment
			} else if ch == '"' || ch == '\'' {
				tokenStart = pos
				quoteChar = ch
				state = yamlString
			} else {
				tokenStart = pos
				state = yamlScalar
			}

		case yamlScalar:
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
					state = yamlLineStart
				} else {
					tokenStart = pos
					state = yamlComment
				}
			}

		case yamlString:
			if escapeNext {
				escapeNext = false
			} else if ch == '\\' && quoteChar == '"' {
				escapeNext = true
			} else if ch == quoteChar {
				h.addToken("string", tokenStart, pos+1)
				state = yamlAfterString
			} else if ch == '\n' && quoteChar == '\'' {
				// Single-quoted strings don't support multi-line in our simplified parser
				h.addToken("string", tokenStart, pos)
				state = yamlLineStart
			}

		case yamlAfterString:
			if ch == '\n' {
				state = yamlLineStart
			} else if ch == '#' {
				tokenStart = pos
				state = yamlComment
			} else if ch != ' ' && ch != '\t' {
				// More content after string
				state = yamlScalar
			}

		case yamlComment:
			if ch == '\n' {
				h.addToken("comment", tokenStart, pos)
				state = yamlLineStart
			}
		}
	}

	// Handle any remaining tokens at end of text
	switch state {
	case yamlComment:
		h.addToken("comment", tokenStart, len(text))
	case yamlString:
		h.addToken("string", tokenStart, len(text))
	case yamlScalar, yamlKeyOrScalar:
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

	state := tomlLineStart
	tokenStart := 0
	quoteChar := byte(0)
	escapeNext := false
	bracketDepth := 0

	for pos := 0; pos < len(text); pos++ {
		ch := text[pos]

		switch state {
		case tomlLineStart:
			if ch == ' ' || ch == '\t' {
				// Skip indentation
			} else if ch == '#' {
				tokenStart = pos
				state = tomlComment
			} else if ch == '[' {
				tokenStart = pos
				bracketDepth = 1
				state = tomlSection
			} else if ch == '\n' {
				// Stay in lineStart
			} else if ch == '"' || ch == '\'' {
				tokenStart = pos
				quoteChar = ch
				state = tomlKeyString
			} else {
				tokenStart = pos
				state = tomlKey
			}

		case tomlSection:
			if ch == '[' {
				bracketDepth++
			} else if ch == ']' {
				bracketDepth--
				if bracketDepth == 0 {
					h.addToken("key", tokenStart, pos+1)
					state = tomlAfterSection
				}
			} else if ch == '\n' {
				// Invalid section header
				state = tomlLineStart
			}

		case tomlAfterSection:
			if ch == '\n' {
				state = tomlLineStart
			} else if ch == '#' {
				tokenStart = pos
				state = tomlComment
			} else if ch != ' ' && ch != '\t' {
				// Invalid content after section
				state = tomlLineStart
			}

		case tomlKey:
			if ch == '=' {
				h.addToken("key", tokenStart, pos)
				state = tomlAfterEquals
			} else if ch == '\n' {
				// Key without value
				state = tomlLineStart
			} else if ch == ' ' || ch == '\t' {
				// End of key, expect equals
				h.addToken("key", tokenStart, pos)
				state = tomlBeforeEquals
			}

		case tomlKeyString:
			if escapeNext {
				escapeNext = false
			} else if ch == '\\' {
				escapeNext = true
			} else if ch == quoteChar {
				state = tomlAfterKeyString
			} else if ch == '\n' && quoteChar == '\'' {
				// Single-quoted strings don't support multi-line
				h.addToken("key", tokenStart, pos)
				state = tomlLineStart
			}

		case tomlAfterKeyString:
			if ch == ' ' || ch == '\t' {
				// Skip whitespace
			} else if ch == '=' {
				h.addToken("key", tokenStart, pos)
				state = tomlAfterEquals
			} else if ch == '\n' {
				h.addToken("key", tokenStart, pos)
				state = tomlLineStart
			}

		case tomlBeforeEquals:
			if ch == ' ' || ch == '\t' {
				// Skip whitespace
			} else if ch == '=' {
				state = tomlAfterEquals
			} else if ch == '\n' {
				state = tomlLineStart
			}

		case tomlAfterEquals:
			if ch == ' ' || ch == '\t' {
				// Skip whitespace
			} else if ch == '\n' {
				state = tomlLineStart
			} else if ch == '#' {
				tokenStart = pos
				state = tomlComment
			} else if ch == '"' || ch == '\'' {
				tokenStart = pos
				quoteChar = ch
				state = tomlValueString
			} else if ch == '[' {
				tokenStart = pos
				state = tomlArray
			} else if ch == '{' {
				tokenStart = pos
				state = tomlInlineTable
			} else if (ch >= '0' && ch <= '9') || ch == '-' || ch == '+' {
				tokenStart = pos
				state = tomlNumber
			} else if ch == 't' || ch == 'f' {
				tokenStart = pos
				state = tomlBoolean
			} else {
				tokenStart = pos
				state = tomlBareString
			}

		case tomlValueString:
			if escapeNext {
				escapeNext = false
			} else if ch == '\\' {
				escapeNext = true
			} else if ch == quoteChar {
				h.addToken("string", tokenStart, pos+1)
				state = tomlAfterValue
			} else if ch == '\n' && quoteChar == '\'' {
				// Single-quoted strings don't support multi-line in our simplified parser
				h.addToken("string", tokenStart, pos)
				state = tomlLineStart
			}

		case tomlBareString:
			if ch == '\n' || ch == '#' || ch == ',' || ch == ']' || ch == '}' {
				h.addToken("string", tokenStart, pos)
				if ch == '\n' {
					state = tomlLineStart
				} else if ch == '#' {
					tokenStart = pos
					state = tomlComment
				} else if ch == ',' {
					state = tomlAfterComma
				} else {
					state = tomlInValue
				}
			}

		case tomlNumber:
			if ch == '\n' || ch == '#' || ch == ',' || ch == ']' || ch == '}' || ch == ' ' || ch == '\t' {
				h.addToken("number", tokenStart, pos)
				if ch == '\n' {
					state = tomlLineStart
				} else if ch == '#' {
					tokenStart = pos
					state = tomlComment
				} else if ch == ',' {
					state = tomlAfterComma
				} else if ch == ']' || ch == '}' {
					state = tomlInValue
				} else {
					state = tomlAfterValue
				}
			} else if (ch >= '0' && ch <= '9') || ch == '.' || ch == 'e' || ch == 'E' || ch == '+' || ch == '-' || ch == '_' || ch == ':' || ch == 'T' || ch == 'Z' {
				// Continue number (including dates/times)
			} else {
				// End of number
				h.addToken("number", tokenStart, pos)
				state = tomlAfterValue
			}

		case tomlBoolean:
			if ch == '\n' || ch == '#' || ch == ',' || ch == ']' || ch == '}' || ch == ' ' || ch == '\t' {
				value := text[tokenStart:pos]
				if value == "true" || value == "false" {
					h.addToken("bool", tokenStart, pos)
				}
				if ch == '\n' {
					state = tomlLineStart
				} else if ch == '#' {
					tokenStart = pos
					state = tomlComment
				} else if ch == ',' {
					state = tomlAfterComma
				} else if ch == ']' || ch == '}' {
					state = tomlInValue
				} else {
					state = tomlAfterValue
				}
			} else if ch >= 'a' && ch <= 'z' {
				// Continue boolean
			} else {
				// Not a boolean
				state = tomlBareString
			}

		case tomlArray:
			if ch == ']' {
				state = tomlAfterValue
			} else if ch == ' ' || ch == '\t' || ch == '\n' {
				// Skip whitespace
			} else if ch == '#' {
				tokenStart = pos
				state = tomlComment
			} else if ch == '"' || ch == '\'' {
				tokenStart = pos
				quoteChar = ch
				state = tomlValueString
			} else if ch == '[' {
				// Nested array
			} else if (ch >= '0' && ch <= '9') || ch == '-' || ch == '+' {
				tokenStart = pos
				state = tomlNumber
			} else if ch == 't' || ch == 'f' {
				tokenStart = pos
				state = tomlBoolean
			} else {
				tokenStart = pos
				state = tomlBareString
			}

		case tomlInlineTable:
			// Simplified inline table handling
			if ch == '}' {
				state = tomlAfterValue
			}

		case tomlAfterComma:
			if ch == ' ' || ch == '\t' || ch == '\n' {
				// Skip whitespace
			} else if ch == '"' || ch == '\'' {
				tokenStart = pos
				quoteChar = ch
				state = tomlValueString
			} else if (ch >= '0' && ch <= '9') || ch == '-' || ch == '+' {
				tokenStart = pos
				state = tomlNumber
			} else if ch == 't' || ch == 'f' {
				tokenStart = pos
				state = tomlBoolean
			} else {
				tokenStart = pos
				state = tomlBareString
			}

		case tomlAfterValue:
			if ch == '\n' {
				state = tomlLineStart
			} else if ch == '#' {
				tokenStart = pos
				state = tomlComment
			} else if ch == ',' {
				state = tomlAfterComma
			} else if ch == ']' || ch == '}' {
				state = tomlAfterValue
			}

		case tomlInValue:
			// Generic state for when we're inside a complex value
			if ch == '\n' {
				state = tomlLineStart
			} else if ch == ']' || ch == '}' {
				state = tomlAfterValue
			} else if ch == ',' {
				state = tomlAfterComma
			}

		case tomlComment:
			if ch == '\n' {
				h.addToken("comment", tokenStart, pos)
				state = tomlLineStart
			}
		}
	}

	// Handle any remaining tokens at end of text
	switch state {
	case tomlComment:
		h.addToken("comment", tokenStart, len(text))
	case tomlValueString, tomlKeyString:
		h.addToken("string", tokenStart, len(text))
	case tomlBareString:
		h.addToken("string", tokenStart, len(text))
	case tomlNumber:
		h.addToken("number", tokenStart, len(text))
	case tomlBoolean:
		value := text[tokenStart:]
		if value == "true" || value == "false" {
			h.addToken("bool", tokenStart, len(text))
		}
	case tomlKey:
		h.addToken("key", tokenStart, len(text))
	case tomlSection:
		h.addToken("key", tokenStart, len(text))
	}

	return h.insertions
}

func highlightJSON(text string, offset int) []insertion {
	h := &syntaxHighlighter{
		text:   text,
		offset: offset,
	}

	state := jsonValue
	tokenStart := 0
	escapeNext := false
	contextStack := []string{}

	for pos := 0; pos < len(text); pos++ {
		ch := text[pos]

		switch state {
		case jsonValue:
			if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
				// Skip whitespace
			} else if ch == '"' {
				tokenStart = pos
				state = jsonString
			} else if ch == '{' {
				contextStack = append(contextStack, "object")
				state = jsonObjectStart
			} else if ch == '[' {
				contextStack = append(contextStack, "array")
				state = jsonValue
			} else if ch == 't' || ch == 'f' || ch == 'n' {
				tokenStart = pos
				state = jsonKeyword
			} else if ch == '-' || (ch >= '0' && ch <= '9') {
				tokenStart = pos
				state = jsonNumber
			} else if ch == '}' || ch == ']' {
				if len(contextStack) > 0 {
					contextStack = contextStack[:len(contextStack)-1]
				}
				if len(contextStack) == 0 {
					state = jsonDone
				} else {
					state = jsonAfterValue
				}
			}

		case jsonObjectStart:
			if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
				// Skip whitespace
			} else if ch == '"' {
				tokenStart = pos
				state = jsonObjectKey
			} else if ch == '}' {
				if len(contextStack) > 0 {
					contextStack = contextStack[:len(contextStack)-1]
				}
				if len(contextStack) == 0 {
					state = jsonDone
				} else {
					state = jsonAfterValue
				}
			}

		case jsonObjectKey:
			if escapeNext {
				escapeNext = false
			} else if ch == '\\' {
				escapeNext = true
			} else if ch == '"' {
				h.addToken("key", tokenStart, pos+1)
				state = jsonAfterKey
			}

		case jsonAfterKey:
			if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
				// Skip whitespace
			} else if ch == ':' {
				state = jsonValue
			}

		case jsonString:
			if escapeNext {
				escapeNext = false
			} else if ch == '\\' {
				escapeNext = true
			} else if ch == '"' {
				h.addToken("string", tokenStart, pos+1)
				state = jsonAfterValue
			}

		case jsonNumber:
			if ch == ',' || ch == '}' || ch == ']' || ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
				h.addToken("number", tokenStart, pos)
				if ch == ',' {
					context := ""
					if len(contextStack) > 0 {
						context = contextStack[len(contextStack)-1]
					}
					if context == "object" {
						state = jsonExpectKey
					} else {
						state = jsonValue
					}
				} else if ch == '}' || ch == ']' {
					if len(contextStack) > 0 {
						contextStack = contextStack[:len(contextStack)-1]
					}
					if len(contextStack) == 0 {
						state = jsonDone
					} else {
						state = jsonAfterValue
					}
				} else {
					state = jsonAfterValue
				}
			} else if (ch >= '0' && ch <= '9') || ch == '.' || ch == 'e' || ch == 'E' || ch == '+' || ch == '-' {
				// Continue number
			} else {
				// Invalid number
				state = jsonError
			}

		case jsonKeyword:
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
						state = jsonExpectKey
					} else {
						state = jsonValue
					}
				} else if ch == '}' || ch == ']' {
					if len(contextStack) > 0 {
						contextStack = contextStack[:len(contextStack)-1]
					}
					if len(contextStack) == 0 {
						state = jsonDone
					} else {
						state = jsonAfterValue
					}
				} else {
					state = jsonAfterValue
				}
			} else if ch >= 'a' && ch <= 'z' {
				// Continue keyword
			} else {
				// Invalid keyword
				state = jsonError
			}

		case jsonAfterValue:
			if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
				// Skip whitespace
			} else if ch == ',' {
				context := ""
				if len(contextStack) > 0 {
					context = contextStack[len(contextStack)-1]
				}
				if context == "object" {
					state = jsonExpectKey
				} else {
					state = jsonValue
				}
			} else if ch == '}' || ch == ']' {
				if len(contextStack) > 0 {
					contextStack = contextStack[:len(contextStack)-1]
				}
				if len(contextStack) == 0 {
					state = jsonDone
				} else {
					state = jsonAfterValue
				}
			}

		case jsonExpectKey:
			if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
				// Skip whitespace
			} else if ch == '"' {
				tokenStart = pos
				state = jsonObjectKey
			} else if ch == '}' {
				if len(contextStack) > 0 {
					contextStack = contextStack[:len(contextStack)-1]
				}
				if len(contextStack) == 0 {
					state = jsonDone
				} else {
					state = jsonAfterValue
				}
			}

		case jsonError, jsonDone:
			// Stop processing
			return h.insertions
		}
	}

	// Handle any remaining tokens at end of text
	switch state {
	case jsonString:
		h.addToken("string", tokenStart, len(text))
	case jsonObjectKey:
		h.addToken("key", tokenStart, len(text))
	case jsonNumber:
		h.addToken("number", tokenStart, len(text))
	case jsonKeyword:
		keyword := text[tokenStart:]
		if keyword == "true" || keyword == "false" {
			h.addToken("bool", tokenStart, len(text))
		} else if keyword == "null" {
			h.addToken("keyword", tokenStart, len(text))
		}
	}

	return h.insertions
}

// Shell state machine states
type shellState int

const (
	shellStart shellState = iota
	shellWord
	shellSingleQuote
	shellDoubleQuote
	shellBacktick
	shellComment
	shellAfterWord
	shellHeredocMarker
)

func highlightShell(text string, offset int) []insertion {
	h := &syntaxHighlighter{
		text:   text,
		offset: offset,
	}

	state := shellStart
	tokenStart := 0
	wordStart := 0
	escapeNext := false
	isFirstWord := true
	sawPrompt := false

	for pos := 0; pos < len(text); pos++ {
		ch := text[pos]

		switch state {
		case shellStart:
			if ch == ' ' || ch == '\t' {
				// Skip whitespace
			} else if ch == '\n' {
				isFirstWord = true
				sawPrompt = false
			} else if ch == '$' && pos+1 < len(text) && text[pos+1] == ' ' && isFirstWord {
				// Shell prompt
				h.addToken("prompt", pos, pos+1)
				sawPrompt = true
				pos++ // Skip the space after prompt
				isFirstWord = true
			} else if ch == '#' {
				tokenStart = pos
				state = shellComment
			} else if ch == '\'' {
				tokenStart = pos
				state = shellSingleQuote
			} else if ch == '"' {
				tokenStart = pos
				state = shellDoubleQuote
			} else if ch == '`' {
				tokenStart = pos
				state = shellBacktick
			} else if ch == '&' && pos+1 < len(text) && text[pos+1] == 'g' {
				// This is &gt; (escaped >)
				h.addToken("operator", pos, pos+4)
				pos += 3
				isFirstWord = true
			} else if ch == '&' && pos+1 < len(text) && text[pos+1] == 'l' {
				// This is &lt; (escaped <)
				h.addToken("operator", pos, pos+4)
				pos += 3
				isFirstWord = true
				// Check for &lt;&lt; (heredoc)
				if pos < len(text) && text[pos] == '&' && pos+3 < len(text) && text[pos:pos+4] == "&lt;" {
					h.addToken("operator", pos, pos+4)
					pos += 4
					// Skip any whitespace
					for pos < len(text) && (text[pos] == ' ' || text[pos] == '\t') {
						pos++
					}
					state = shellHeredocMarker
					wordStart = pos
				}
			} else if ch == '&' && pos+1 < len(text) && text[pos+1] == 'a' {
				// This is &amp; (escaped &)
				h.addToken("operator", pos, pos+5)
				pos += 4
				isFirstWord = true
			} else if ch == '|' || ch == ';' {
				h.addToken("operator", pos, pos+1)
				isFirstWord = true
			} else if ch == '<' || ch == '>' {
				// Skip HTML tags entirely
				for pos < len(text) && text[pos] != '>' {
					pos++
				}
				// Don't highlight HTML tags
			} else if ch == '-' && isFirstWord && sawPrompt {
				// This is a flag
				wordStart = pos
				state = shellWord
			} else {
				// Start of a word
				wordStart = pos
				state = shellWord
			}

		case shellWord:
			if ch == ' ' || ch == '\t' || ch == '\n' || ch == '|' || ch == ';' ||
				ch == '\'' || ch == '"' || ch == '`' || ch == '#' {
				// End of word
				word := text[wordStart:pos]

				if isFirstWord && sawPrompt {
					if isShellKeyword(word) {
						h.addToken("keyword", wordStart, pos)
					} else if isShellBuiltin(word) {
						h.addToken("builtin", wordStart, pos)
					} else {
						h.addToken("command", wordStart, pos)
					}
					isFirstWord = false
				} else if len(word) > 0 && word[0] == '-' {
					h.addToken("flag", wordStart, pos)
				} else if len(word) > 0 && word[0] == '$' {
					h.addToken("variable", wordStart, pos)
				} else if len(word) > 0 {
					h.addToken("argument", wordStart, pos)
				}

				state = shellAfterWord
				pos-- // Reprocess this character
			} else if ch == '&' && pos+2 < len(text) && text[pos:pos+3] == "&gt" {
				// End word before operator
				word := text[wordStart:pos]
				if len(word) > 0 && word[0] == '-' {
					h.addToken("flag", wordStart, pos)
				} else if len(word) > 0 {
					h.addToken("argument", wordStart, pos)
				}
				state = shellStart
				pos-- // Reprocess
			} else if ch == '&' && pos+2 < len(text) && text[pos:pos+3] == "&lt" {
				// End word before operator
				word := text[wordStart:pos]
				if len(word) > 0 && word[0] == '-' {
					h.addToken("flag", wordStart, pos)
				} else if len(word) > 0 {
					h.addToken("argument", wordStart, pos)
				}
				state = shellStart
				pos-- // Reprocess
			} else if ch == '<' || ch == '>' {
				// End word before HTML tag
				if pos > wordStart {
					word := text[wordStart:pos]
					if word[0] == '-' {
						h.addToken("flag", wordStart, pos)
					} else {
						h.addToken("argument", wordStart, pos)
					}
				}
				state = shellStart
				pos-- // Reprocess
			}

		case shellAfterWord:
			if ch == ' ' || ch == '\t' {
				// Skip whitespace
			} else if ch == '\n' {
				isFirstWord = true
				sawPrompt = false
				state = shellStart
			} else {
				state = shellStart
				pos-- // Reprocess
			}

		case shellSingleQuote:
			if ch == '\'' {
				h.addToken("string", tokenStart, pos+1)
				state = shellAfterWord
			}

		case shellDoubleQuote:
			if escapeNext {
				escapeNext = false
			} else if ch == '\\' {
				escapeNext = true
			} else if ch == '"' {
				h.addToken("string", tokenStart, pos+1)
				state = shellAfterWord
			} else if ch == '$' && pos+1 < len(text) && isShellVarChar(text[pos+1]) {
				// Variable inside double quotes
				varStart := pos
				pos++
				for pos < len(text) && isShellVarChar(text[pos]) {
					pos++
				}
				h.addToken("variable", varStart, pos)
				pos-- // Back up one
			}

		case shellBacktick:
			if escapeNext {
				escapeNext = false
			} else if ch == '\\' {
				escapeNext = true
			} else if ch == '`' {
				h.addToken("string", tokenStart, pos+1)
				state = shellAfterWord
			}

		case shellComment:
			if ch == '\n' {
				h.addToken("comment", tokenStart, pos)
				isFirstWord = true
				sawPrompt = false
				state = shellStart
			}

		case shellHeredocMarker:
			// Handle heredoc marker like 'EOF' or <<'EOF'
			if ch == '\n' || ch == ' ' || ch == '\t' {
				// End of heredoc marker
				h.addToken("string", wordStart, pos)
				if ch == '\n' {
					isFirstWord = true
					sawPrompt = false
				}
				state = shellStart
			} else if ch == '\'' && wordStart == pos {
				// Quoted heredoc like <<'EOF'
				tokenStart = pos
				state = shellSingleQuote
			}
		}
	}

	// Handle any remaining tokens at end of text
	switch state {
	case shellComment:
		h.addToken("comment", tokenStart, len(text))
	case shellSingleQuote, shellDoubleQuote, shellBacktick:
		h.addToken("string", tokenStart, len(text))
	case shellWord:
		word := text[wordStart:]
		if isFirstWord && sawPrompt {
			if isShellKeyword(word) {
				h.addToken("keyword", wordStart, len(text))
			} else if isShellBuiltin(word) {
				h.addToken("builtin", wordStart, len(text))
			} else {
				h.addToken("command", wordStart, len(text))
			}
		} else if word[0] == '-' {
			h.addToken("flag", wordStart, len(text))
		} else if word[0] == '$' {
			h.addToken("variable", wordStart, len(text))
		} else {
			h.addToken("argument", wordStart, len(text))
		}
	}

	return h.insertions
}

func isShellVarChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

func isLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isShellKeyword(word string) bool {
	keywords := []string{
		"if", "then", "else", "elif", "fi", "case", "esac", "for", "while", "until",
		"do", "done", "function", "return", "break", "continue", "shift", "exit",
		"export", "local", "readonly", "declare", "typeset", "set", "unset", "source",
	}
	return slices.Contains(keywords, word)
}

func isShellBuiltin(word string) bool {
	builtins := []string{
		"echo", "cd", "pwd", "ls", "cp", "mv", "rm", "mkdir", "rmdir", "touch",
		"cat", "grep", "sed", "awk", "cut", "sort", "uniq", "head", "tail", "wc",
		"find", "xargs", "curl", "wget", "tar", "gzip", "gunzip", "zip", "unzip",
		"chmod", "chown", "ln", "ps", "kill", "bg", "fg", "jobs", "alias", "unalias",
		"which", "whereis", "date", "cal", "sleep", "test", "[", "[[", "]]",
	}
	return slices.Contains(builtins, word)
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
