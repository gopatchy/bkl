package main

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
			switch ch {
			case ' ', '\t':
				// Skip indentation
			case '#':
				tokenStart = pos
				state = tomlComment
			case '[':
				tokenStart = pos
				bracketDepth = 1
				state = tomlSection
			case '\n':
				// Stay in lineStart
			case '"', '\'':
				tokenStart = pos
				quoteChar = ch
				state = tomlKeyString
			default:
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
			switch {
			case ch == ' ' || ch == '\t':
				// Skip whitespace
			case ch == '\n':
				state = tomlLineStart
			case ch == '#':
				tokenStart = pos
				state = tomlComment
			case ch == '"' || ch == '\'':
				tokenStart = pos
				quoteChar = ch
				state = tomlValueString
			case ch == '[':
				tokenStart = pos
				state = tomlArray
			case ch == '{':
				tokenStart = pos
				state = tomlInlineTable
			case (ch >= '0' && ch <= '9') || ch == '-' || ch == '+':
				tokenStart = pos
				state = tomlNumber
			case ch == 't' || ch == 'f':
				tokenStart = pos
				state = tomlBoolean
			default:
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
				switch ch {
				case '\n':
					state = tomlLineStart
				case '#':
					tokenStart = pos
					state = tomlComment
				case ',':
					state = tomlAfterComma
				default:
					state = tomlInValue
				}
			}

		case tomlNumber:
			if ch == '\n' || ch == '#' || ch == ',' || ch == ']' || ch == '}' || ch == ' ' || ch == '\t' {
				h.addToken("number", tokenStart, pos)
				switch ch {
				case '\n':
					state = tomlLineStart
				case '#':
					tokenStart = pos
					state = tomlComment
				case ',':
					state = tomlAfterComma
				case ']', '}':
					state = tomlInValue
				default:
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
				switch ch {
				case '\n':
					state = tomlLineStart
				case '#':
					tokenStart = pos
					state = tomlComment
				case ',':
					state = tomlAfterComma
				case ']', '}':
					state = tomlInValue
				default:
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
		switch value {
		case "true", "false":
			h.addToken("bool", tokenStart, len(text))
		}
	case tomlKey:
		h.addToken("key", tokenStart, len(text))
	case tomlSection:
		h.addToken("key", tokenStart, len(text))
	}

	return h.insertions
}
