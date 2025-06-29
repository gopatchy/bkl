package main

// Diff state machine states
type diffState int

const (
	diffLineStart diffState = iota
	diffMaybePlus
	diffMaybePlusPlus
	diffMaybeMinus
	diffMaybeMinusMinus
	diffMaybeHunk
	diffAddition
	diffDeletion
	diffFileHeader
	diffHunkHeader
	diffContext
)

func highlightDiff(text string, offset int) []insertion {
	h := &syntaxHighlighter{
		text:   text,
		offset: offset,
	}

	state := diffLineStart
	tokenStart := 0

	for pos := 0; pos < len(text); pos++ {
		ch := text[pos]

		switch state {
		case diffLineStart:
			switch ch {
			case '+':
				tokenStart = pos
				state = diffMaybePlus
			case '-':
				tokenStart = pos
				state = diffMaybeMinus
			case '@':
				tokenStart = pos
				state = diffMaybeHunk
			case '\n':
				// Stay in lineStart
			case ' ':
				// Context line
				tokenStart = pos
				state = diffContext
			default:
				// Also context line
				tokenStart = pos
				state = diffContext
			}

		case diffMaybePlus:
			if pos == tokenStart+1 && ch == '+' {
				state = diffMaybePlusPlus
			} else {
				// Regular addition line
				state = diffAddition
			}

		case diffMaybePlusPlus:
			if pos == tokenStart+2 && ch == '+' {
				// File header
				state = diffFileHeader
			} else {
				// Was just ++ at start, treat as addition
				state = diffAddition
			}

		case diffMaybeMinus:
			if pos == tokenStart+1 && ch == '-' {
				state = diffMaybeMinusMinus
			} else {
				// Regular deletion line
				state = diffDeletion
			}

		case diffMaybeMinusMinus:
			if pos == tokenStart+2 && ch == '-' {
				// File header
				state = diffFileHeader
			} else {
				// Was just -- at start, treat as deletion
				state = diffDeletion
			}

		case diffMaybeHunk:
			if pos == tokenStart+1 && ch == '@' {
				// Hunk header
				state = diffHunkHeader
			} else {
				// Just @ at start, treat as context
				state = diffContext
			}

		case diffAddition:
			if ch == '\n' {
				h.addToken("addition", tokenStart, pos)
				state = diffLineStart
			}

		case diffDeletion:
			if ch == '\n' {
				h.addToken("deletion", tokenStart, pos)
				state = diffLineStart
			}

		case diffFileHeader:
			if ch == '\n' {
				h.addToken("file-header", tokenStart, pos)
				state = diffLineStart
			}

		case diffHunkHeader:
			if ch == '\n' {
				h.addToken("hunk-header", tokenStart, pos)
				state = diffLineStart
			}

		case diffContext:
			if ch == '\n' {
				// Don't highlight context lines
				state = diffLineStart
			}
		}
	}

	// Handle any remaining tokens at end of text
	switch state {
	case diffAddition:
		h.addToken("addition", tokenStart, len(text))
	case diffDeletion:
		h.addToken("deletion", tokenStart, len(text))
	case diffFileHeader:
		h.addToken("file-header", tokenStart, len(text))
	case diffHunkHeader:
		h.addToken("hunk-header", tokenStart, len(text))
	}

	return h.insertions
}
