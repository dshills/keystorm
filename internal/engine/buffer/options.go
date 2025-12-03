package buffer

// Option is a functional option for configuring a Buffer.
type Option func(*Buffer)

// WithLineEnding sets the buffer's line ending style.
func WithLineEnding(le LineEnding) Option {
	return func(b *Buffer) {
		b.lineEnding = le
	}
}

// WithTabWidth sets the buffer's tab width.
func WithTabWidth(width int) Option {
	return func(b *Buffer) {
		if width > 0 {
			b.tabWidth = width
		}
	}
}

// WithLF configures the buffer to use Unix line endings (\n).
func WithLF() Option {
	return WithLineEnding(LineEndingLF)
}

// WithCRLF configures the buffer to use Windows line endings (\r\n).
func WithCRLF() Option {
	return WithLineEnding(LineEndingCRLF)
}

// WithCR configures the buffer to use old Mac line endings (\r).
func WithCR() Option {
	return WithLineEnding(LineEndingCR)
}

// DetectLineEnding returns a LineEnding based on the most common line ending in the text.
// Returns LineEndingLF if no line endings are found.
func DetectLineEnding(text string) LineEnding {
	var lfCount, crlfCount, crCount int

	i := 0
	for i < len(text) {
		if i+1 < len(text) && text[i] == '\r' && text[i+1] == '\n' {
			crlfCount++
			i += 2
		} else if text[i] == '\r' {
			crCount++
			i++
		} else if text[i] == '\n' {
			lfCount++
			i++
		} else {
			i++
		}
	}

	// Return the most common line ending
	if crlfCount >= lfCount && crlfCount >= crCount {
		if crlfCount > 0 {
			return LineEndingCRLF
		}
	}
	if crCount >= lfCount && crCount >= crlfCount {
		if crCount > 0 {
			return LineEndingCR
		}
	}

	return LineEndingLF
}

// WithDetectedLineEnding sets the buffer's line ending style based on content.
// Call this after creating the buffer with content.
func WithDetectedLineEnding(text string) Option {
	return WithLineEnding(DetectLineEnding(text))
}
