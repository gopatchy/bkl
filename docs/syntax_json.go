package main

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
