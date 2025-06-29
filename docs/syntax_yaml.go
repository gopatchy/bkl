package main

import (
	"strings"
)

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
