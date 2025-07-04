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
			switch {
			case ch == ' ' || ch == '\t':
				// Skip whitespace
			case ch == '\n':
				isFirstWord = true
				sawPrompt = false
			case ch == '$' && pos+1 < len(text) && text[pos+1] == ' ' && isFirstWord:
				// Shell prompt - include the space
				h.addToken("prompt", pos, pos+2)
				sawPrompt = true
				pos++ // Skip the space after prompt
				isFirstWord = true
			case ch == '#':
				tokenStart = pos
				state = shellComment
			case ch == '\'':
				tokenStart = pos
				state = shellSingleQuote
			case ch == '"':
				tokenStart = pos
				state = shellDoubleQuote
			case ch == '`':
				tokenStart = pos
				state = shellBacktick
			case ch == '&' && pos+1 < len(text) && text[pos+1] == 'g':
				// This is &gt; (escaped >)
				h.addToken("operator", pos, pos+4)
				pos += 3
				isFirstWord = true
			case ch == '&' && pos+1 < len(text) && text[pos+1] == 'l':
				// This is &lt; (escaped <)
				h.addToken("operator", pos, pos+4)
				pos += 3
				// Check for &lt;( (process substitution)
				if pos < len(text)-1 && text[pos+1] == '(' {
					// After <(, the next word is a command
					isFirstWord = true
					sawPrompt = true
				}
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
			case ch == '&' && pos+1 < len(text) && text[pos+1] == 'a':
				// This is &amp; (escaped &)
				h.addToken("operator", pos, pos+5)
				pos += 4
				isFirstWord = true
			case ch == '|' || ch == ';' || ch == '(' || ch == ')':
				h.addToken("operator", pos, pos+1)
				isFirstWord = true
				// For '(', treat the next word as a potential command
				if ch == '(' {
					sawPrompt = true
				}
			case ch == '<' || ch == '>':
				// Skip HTML tags entirely
				for pos < len(text) && text[pos] != '>' {
					pos++
				}
				// Don't highlight HTML tags
			case ch == '-' && isFirstWord && sawPrompt:
				// This is a flag
				wordStart = pos
				state = shellWord
			default:
				// Start of a word
				wordStart = pos
				state = shellWord
			}

		case shellWord:
			if ch == ' ' || ch == '\t' || ch == '\n' || ch == '|' || ch == ';' ||
				ch == '\'' || ch == '"' || ch == '`' || ch == '#' || ch == '(' || ch == ')' {
				// End of word
				word := text[wordStart:pos]

				switch {
				case isFirstWord && sawPrompt:
					switch {
					case isShellKeyword(word):
						h.addToken("keyword", wordStart, pos)
					case isShellBuiltin(word):
						h.addToken("command", wordStart, pos)
					default:
						h.addToken("command", wordStart, pos)
					}
					isFirstWord = false
				case len(word) > 0 && word[0] == '-':
					h.addToken("flag", wordStart, pos)
				case len(word) > 0 && word[0] == '$':
					h.addToken("variable", wordStart, pos)
				case len(word) > 0:
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
			switch ch {
			case ' ', '\t':
				// Skip whitespace
			case '\n':
				isFirstWord = true
				sawPrompt = false
				state = shellStart
			case '&':
				// Check if this is the start of &lt;(
				if pos+3 < len(text) && text[pos:pos+4] == "&lt;" &&
					pos+4 < len(text) && text[pos+4] == '(' {
					// This is <( after a word, treat next as command
					state = shellStart
					pos-- // Reprocess
				} else {
					state = shellStart
					pos-- // Reprocess
				}
			default:
				state = shellStart
				pos-- // Reprocess
			}

		case shellSingleQuote:
			if ch == '\'' {
				h.addToken("string", tokenStart, pos+1)
				state = shellAfterWord
			}

		case shellDoubleQuote:
			switch {
			case escapeNext:
				escapeNext = false
			case ch == '\\':
				escapeNext = true
			case ch == '"':
				h.addToken("string", tokenStart, pos+1)
				state = shellAfterWord
			case ch == '$' && pos+1 < len(text) && isShellVarChar(text[pos+1]):
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
			switch {
			case escapeNext:
				escapeNext = false
			case ch == '\\':
				escapeNext = true
			case ch == '`':
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
		switch {
		case isFirstWord && sawPrompt:
			switch {
			case isShellKeyword(word):
				h.addToken("keyword", wordStart, len(text))
			case isShellBuiltin(word):
				h.addToken("command", wordStart, len(text))
			default:
				h.addToken("command", wordStart, len(text))
			}
		case word[0] == '-':
			h.addToken("flag", wordStart, len(text))
		case word[0] == '$':
			h.addToken("variable", wordStart, len(text))
		default:
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
		"yq", "jq", "bkl", "bkld", "bkli", "bklr", "diff", "git", "make", "go", "npm", "python",
	}
	return slices.Contains(builtins, word)
}
