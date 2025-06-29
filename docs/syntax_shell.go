package main

import (
	"slices"
)

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
