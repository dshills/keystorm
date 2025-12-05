package vfs

import (
	"bytes"
	"unicode/utf8"
)

// Encoding represents a character encoding.
type Encoding string

const (
	// EncodingUTF8 is UTF-8 encoding (default).
	EncodingUTF8 Encoding = "utf-8"

	// EncodingUTF8BOM is UTF-8 encoding with BOM.
	EncodingUTF8BOM Encoding = "utf-8-bom"

	// EncodingUTF16LE is UTF-16 Little Endian.
	EncodingUTF16LE Encoding = "utf-16le"

	// EncodingUTF16BE is UTF-16 Big Endian.
	EncodingUTF16BE Encoding = "utf-16be"

	// EncodingLatin1 is ISO-8859-1 (Latin-1).
	EncodingLatin1 Encoding = "iso-8859-1"

	// EncodingASCII is ASCII encoding.
	EncodingASCII Encoding = "ascii"
)

// LineEnding represents the line ending style.
type LineEnding string

const (
	// LineEndingLF is Unix-style line ending (\n).
	LineEndingLF LineEnding = "lf"

	// LineEndingCRLF is Windows-style line ending (\r\n).
	LineEndingCRLF LineEnding = "crlf"

	// LineEndingCR is old Mac-style line ending (\r).
	LineEndingCR LineEnding = "cr"

	// LineEndingMixed indicates mixed line endings.
	LineEndingMixed LineEnding = "mixed"
)

// BOM (Byte Order Mark) constants
var (
	bomUTF8    = []byte{0xEF, 0xBB, 0xBF}
	bomUTF16LE = []byte{0xFF, 0xFE}
	bomUTF16BE = []byte{0xFE, 0xFF}
)

// DetectEncoding attempts to detect the encoding of file content.
// It checks for BOM markers first, then validates UTF-8.
// Falls back to Latin-1 which accepts all byte sequences.
func DetectEncoding(content []byte) Encoding {
	if len(content) == 0 {
		return EncodingUTF8
	}

	// Check for BOM markers
	if bytes.HasPrefix(content, bomUTF8) {
		return EncodingUTF8BOM
	}
	if bytes.HasPrefix(content, bomUTF16LE) {
		return EncodingUTF16LE
	}
	if bytes.HasPrefix(content, bomUTF16BE) {
		return EncodingUTF16BE
	}

	// Check if it's valid UTF-8
	if utf8.Valid(content) {
		// Check if it's pure ASCII
		if isASCII(content) {
			return EncodingASCII
		}
		return EncodingUTF8
	}

	// Fall back to Latin-1 (accepts all bytes)
	return EncodingLatin1
}

// DetectLineEnding detects the dominant line ending in content.
// Returns LineEndingMixed if multiple styles are found with similar frequency.
func DetectLineEnding(content []byte) LineEnding {
	if len(content) == 0 {
		return LineEndingLF // Default to LF
	}

	var lf, crlf, cr int

	for i := 0; i < len(content); i++ {
		if content[i] == '\r' {
			if i+1 < len(content) && content[i+1] == '\n' {
				crlf++
				i++ // Skip the \n
			} else {
				cr++
			}
		} else if content[i] == '\n' {
			lf++
		}
	}

	total := lf + crlf + cr
	if total == 0 {
		return LineEndingLF // Default to LF
	}

	// Check for mixed line endings (more than one style with significant presence)
	count := 0
	if lf > 0 {
		count++
	}
	if crlf > 0 {
		count++
	}
	if cr > 0 {
		count++
	}

	// If multiple styles are present and each has at least 10% of total, it's mixed
	threshold := total / 10
	if threshold < 1 {
		threshold = 1
	}
	mixedCount := 0
	if lf >= threshold {
		mixedCount++
	}
	if crlf >= threshold {
		mixedCount++
	}
	if cr >= threshold {
		mixedCount++
	}
	if mixedCount > 1 {
		return LineEndingMixed
	}

	// Return the dominant style
	if crlf >= lf && crlf >= cr {
		return LineEndingCRLF
	}
	if cr > lf {
		return LineEndingCR
	}
	return LineEndingLF
}

// StripBOM removes the BOM from content if present.
// Returns the content without BOM and the detected encoding.
func StripBOM(content []byte) ([]byte, Encoding) {
	if bytes.HasPrefix(content, bomUTF8) {
		return content[3:], EncodingUTF8BOM
	}
	if bytes.HasPrefix(content, bomUTF16LE) {
		return content[2:], EncodingUTF16LE
	}
	if bytes.HasPrefix(content, bomUTF16BE) {
		return content[2:], EncodingUTF16BE
	}
	return content, EncodingUTF8
}

// AddBOM adds a BOM marker for the specified encoding.
// Only UTF-8 BOM, UTF-16 LE, and UTF-16 BE BOMs are supported.
func AddBOM(content []byte, encoding Encoding) []byte {
	switch encoding {
	case EncodingUTF8BOM:
		if !bytes.HasPrefix(content, bomUTF8) {
			return append(bomUTF8, content...)
		}
	case EncodingUTF16LE:
		if !bytes.HasPrefix(content, bomUTF16LE) {
			return append(bomUTF16LE, content...)
		}
	case EncodingUTF16BE:
		if !bytes.HasPrefix(content, bomUTF16BE) {
			return append(bomUTF16BE, content...)
		}
	}
	return content
}

// NormalizeLineEndings converts all line endings to the specified style.
func NormalizeLineEndings(content []byte, ending LineEnding) []byte {
	if len(content) == 0 {
		return content
	}

	var newline []byte
	switch ending {
	case LineEndingLF:
		newline = []byte{'\n'}
	case LineEndingCRLF:
		newline = []byte{'\r', '\n'}
	case LineEndingCR:
		newline = []byte{'\r'}
	default:
		return content // Don't normalize mixed
	}

	// First normalize everything to LF
	result := make([]byte, 0, len(content))
	for i := 0; i < len(content); i++ {
		if content[i] == '\r' {
			if i+1 < len(content) && content[i+1] == '\n' {
				i++ // Skip the \n
			}
			result = append(result, '\n')
		} else {
			result = append(result, content[i])
		}
	}

	// If target is LF, we're done
	if ending == LineEndingLF {
		return result
	}

	// Convert LF to target
	return bytes.ReplaceAll(result, []byte{'\n'}, newline)
}

// CountLines counts the number of lines in content.
func CountLines(content []byte) int {
	if len(content) == 0 {
		return 0
	}

	lines := 1
	for i := 0; i < len(content); i++ {
		if content[i] == '\r' {
			lines++
			if i+1 < len(content) && content[i+1] == '\n' {
				i++ // Skip the \n in CRLF
			}
		} else if content[i] == '\n' {
			lines++
		}
	}

	// Don't count trailing newline as extra line
	lastByte := content[len(content)-1]
	if lastByte == '\n' || lastByte == '\r' {
		lines--
	}

	return lines
}

// IsBinary attempts to detect if content is binary (not text).
// Uses heuristics: presence of null bytes, high ratio of non-printable characters.
func IsBinary(content []byte) bool {
	if len(content) == 0 {
		return false
	}

	// Check first 8KB at most
	checkLen := len(content)
	if checkLen > 8192 {
		checkLen = 8192
	}

	sample := content[:checkLen]

	// Null bytes are a strong indicator of binary
	if bytes.Contains(sample, []byte{0}) {
		return true
	}

	// Count non-text bytes (control characters except tab, newline, carriage return)
	nonText := 0
	for _, b := range sample {
		if b < 32 && b != '\t' && b != '\n' && b != '\r' {
			nonText++
		}
	}

	// If more than 10% are non-text, consider it binary
	return float64(nonText)/float64(checkLen) > 0.1
}

// isASCII returns true if all bytes are ASCII (< 128).
func isASCII(content []byte) bool {
	for _, b := range content {
		if b >= 128 {
			return false
		}
	}
	return true
}

// EncodingInfo holds detected encoding information for a file.
type EncodingInfo struct {
	// Encoding is the detected character encoding.
	Encoding Encoding

	// LineEnding is the detected line ending style.
	LineEnding LineEnding

	// HasBOM indicates if the file has a BOM marker.
	HasBOM bool

	// IsBinary indicates if the file appears to be binary.
	IsBinary bool

	// LineCount is the number of lines.
	LineCount int
}

// DetectEncodingInfo performs full encoding detection on content.
func DetectEncodingInfo(content []byte) EncodingInfo {
	info := EncodingInfo{}

	// Check for binary
	info.IsBinary = IsBinary(content)
	if info.IsBinary {
		return info
	}

	// Detect encoding
	info.Encoding = DetectEncoding(content)

	// Check for BOM
	info.HasBOM = info.Encoding == EncodingUTF8BOM ||
		info.Encoding == EncodingUTF16LE ||
		info.Encoding == EncodingUTF16BE

	// Detect line endings
	info.LineEnding = DetectLineEnding(content)

	// Count lines
	info.LineCount = CountLines(content)

	return info
}
