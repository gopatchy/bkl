package main

import (
	"slices"
	"strings"
)

// Dockerfile state machine states
type dockerfileState int

const (
	dockerfileLineStart dockerfileState = iota
	dockerfileInstruction
	dockerfileAfterInstruction
	dockerfileArgument
	dockerfileString
	dockerfileComment
	dockerfileVariable
	dockerfileEscape
)

func highlightDockerfile(text string, offset int) []insertion {
	h := &syntaxHighlighter{
		text:   text,
		offset: offset,
	}

	state := dockerfileLineStart
	tokenStart := 0
	quoteChar := byte(0)
	escapeNext := false
	lineIsComment := false

	for pos := 0; pos < len(text); pos++ {
		ch := text[pos]

		switch state {
		case dockerfileLineStart:
			switch {
			case ch == ' ' || ch == '\t':
				// Skip whitespace
			case ch == '#':
				tokenStart = pos
				state = dockerfileComment
				lineIsComment = true
			case ch == '\n':
				// Stay in lineStart
				lineIsComment = false
			case isDockerfileLetter(ch):
				tokenStart = pos
				state = dockerfileInstruction
			default:
				// Unexpected character at line start, treat as argument
				tokenStart = pos
				state = dockerfileArgument
			}

		case dockerfileInstruction:
			if ch == ' ' || ch == '\t' || ch == '\n' {
				instruction := strings.ToUpper(text[tokenStart:pos])
				if isDockerfileInstruction(instruction) {
					h.addToken("keyword", tokenStart, pos)
				} else {
					// Not an instruction, treat as argument
					h.addToken("argument", tokenStart, pos)
				}

				if ch == '\n' {
					state = dockerfileLineStart
					lineIsComment = false
				} else {
					state = dockerfileAfterInstruction
				}
			}

		case dockerfileAfterInstruction:
			switch {
			case ch == ' ' || ch == '\t':
				// Skip whitespace
			case ch == '\n':
				state = dockerfileLineStart
				lineIsComment = false
			case ch == '#':
				tokenStart = pos
				state = dockerfileComment
			case ch == '"', ch == '\'':
				tokenStart = pos
				quoteChar = ch
				state = dockerfileString
			case ch == '$':
				tokenStart = pos
				state = dockerfileVariable
			case ch == '\\' && pos == len(text)-1:
				// Line continuation at end of file
				h.addToken("escape", pos, pos+1)
			case ch == '\\' && pos+1 < len(text) && text[pos+1] == '\n':
				// Line continuation
				h.addToken("escape", pos, pos+1)
				pos++ // Skip the newline
			default:
				tokenStart = pos
				state = dockerfileArgument
			}

		case dockerfileArgument:
			switch {
			case ch == ' ' || ch == '\t' || ch == '\n':
				if pos > tokenStart {
					h.addToken("argument", tokenStart, pos)
				}
				if ch == '\n' {
					state = dockerfileLineStart
					lineIsComment = false
				} else {
					state = dockerfileAfterInstruction
				}
			case ch == '"', ch == '\'':
				// End current argument and start string
				if pos > tokenStart {
					h.addToken("argument", tokenStart, pos)
				}
				tokenStart = pos
				quoteChar = ch
				state = dockerfileString
			case ch == '$':
				// End current argument and start variable
				if pos > tokenStart {
					h.addToken("argument", tokenStart, pos)
				}
				tokenStart = pos
				state = dockerfileVariable
			case ch == '#' && !lineIsComment:
				// Inline comment
				if pos > tokenStart {
					h.addToken("argument", tokenStart, pos)
				}
				tokenStart = pos
				state = dockerfileComment
			case ch == '\\' && pos == len(text)-1:
				// Line continuation at end of file
				if pos > tokenStart {
					h.addToken("argument", tokenStart, pos)
				}
				h.addToken("escape", pos, pos+1)
			case ch == '\\' && pos+1 < len(text) && text[pos+1] == '\n':
				// Line continuation
				if pos > tokenStart {
					h.addToken("argument", tokenStart, pos)
				}
				h.addToken("escape", pos, pos+1)
				pos++ // Skip the newline
				state = dockerfileAfterInstruction
			}

		case dockerfileString:
			if escapeNext {
				escapeNext = false
			} else if ch == '\\' {
				escapeNext = true
			} else if ch == quoteChar {
				h.addToken("string", tokenStart, pos+1)
				state = dockerfileAfterInstruction
			} else if ch == '\n' && !escapeNext {
				// Unclosed string at end of line
				h.addToken("string", tokenStart, pos)
				state = dockerfileLineStart
				lineIsComment = false
			}

		case dockerfileVariable:
			if ch == '{' && pos == tokenStart+1 {
				// ${VAR} syntax
				for pos < len(text) && text[pos] != '}' {
					pos++
				}
				if pos < len(text) {
					h.addToken("variable", tokenStart, pos+1)
				} else {
					h.addToken("variable", tokenStart, len(text))
				}
				state = dockerfileAfterInstruction
			} else if isDockerfileVarChar(ch) && pos > tokenStart {
				// Continue variable name
			} else {
				// End of variable
				h.addToken("variable", tokenStart, pos)
				state = dockerfileAfterInstruction
				pos-- // Reprocess this character
			}

		case dockerfileComment:
			if ch == '\n' {
				h.addToken("comment", tokenStart, pos)
				state = dockerfileLineStart
				lineIsComment = false
			}

		case dockerfileEscape:
			// This state is not used in current implementation
			// as escape sequences are handled inline
		}
	}

	// Handle any remaining tokens at end of text
	switch state {
	case dockerfileComment:
		h.addToken("comment", tokenStart, len(text))
	case dockerfileString:
		h.addToken("string", tokenStart, len(text))
	case dockerfileArgument:
		h.addToken("argument", tokenStart, len(text))
	case dockerfileVariable:
		h.addToken("variable", tokenStart, len(text))
	case dockerfileInstruction:
		instruction := strings.ToUpper(text[tokenStart:])
		if isDockerfileInstruction(instruction) {
			h.addToken("keyword", tokenStart, len(text))
		} else {
			h.addToken("argument", tokenStart, len(text))
		}
	}

	return h.insertions
}

func isDockerfileLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isDockerfileVarChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') || ch == '_'
}

func isDockerfileInstruction(word string) bool {
	instructions := []string{
		"FROM", "RUN", "CMD", "LABEL", "EXPOSE", "ENV", "ADD", "COPY",
		"ENTRYPOINT", "VOLUME", "USER", "WORKDIR", "ARG", "ONBUILD",
		"STOPSIGNAL", "HEALTHCHECK", "SHELL", "MAINTAINER",
	}
	return slices.Contains(instructions, word)
}
