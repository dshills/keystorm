package vfs

import (
	"bytes"
	"testing"
)

func TestDetectEncoding(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    Encoding
	}{
		{
			name:    "empty",
			content: []byte{},
			want:    EncodingUTF8,
		},
		{
			name:    "ASCII",
			content: []byte("Hello, World!"),
			want:    EncodingASCII,
		},
		{
			name:    "UTF-8",
			content: []byte("Hello, 世界!"),
			want:    EncodingUTF8,
		},
		{
			name:    "UTF-8 BOM",
			content: append([]byte{0xEF, 0xBB, 0xBF}, []byte("Hello")...),
			want:    EncodingUTF8BOM,
		},
		{
			name:    "UTF-16 LE BOM",
			content: []byte{0xFF, 0xFE, 0x48, 0x00},
			want:    EncodingUTF16LE,
		},
		{
			name:    "UTF-16 BE BOM",
			content: []byte{0xFE, 0xFF, 0x00, 0x48},
			want:    EncodingUTF16BE,
		},
		{
			name:    "Latin-1",
			content: []byte{0x80, 0x90, 0xA0}, // Invalid UTF-8
			want:    EncodingLatin1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectEncoding(tt.content)
			if got != tt.want {
				t.Errorf("DetectEncoding() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectLineEnding(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    LineEnding
	}{
		{
			name:    "empty",
			content: []byte{},
			want:    LineEndingLF,
		},
		{
			name:    "no newlines",
			content: []byte("single line"),
			want:    LineEndingLF,
		},
		{
			name:    "LF only",
			content: []byte("line1\nline2\nline3"),
			want:    LineEndingLF,
		},
		{
			name:    "CRLF only",
			content: []byte("line1\r\nline2\r\nline3"),
			want:    LineEndingCRLF,
		},
		{
			name:    "CR only",
			content: []byte("line1\rline2\rline3"),
			want:    LineEndingCR,
		},
		{
			name:    "mixed significant",
			content: []byte("line1\nline2\r\nline3\nline4\r\n"),
			want:    LineEndingMixed,
		},
		{
			name:    "predominantly LF with one CRLF",
			content: []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n13\n14\n15\n16\n17\n18\n19\n20\r\n"),
			want:    LineEndingLF, // 20 LF + 1 CRLF = 21 total, threshold=2, CRLF count=1 < threshold
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectLineEnding(tt.content)
			if got != tt.want {
				t.Errorf("DetectLineEnding() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStripBOM(t *testing.T) {
	tests := []struct {
		name         string
		content      []byte
		wantContent  []byte
		wantEncoding Encoding
	}{
		{
			name:         "no BOM",
			content:      []byte("Hello"),
			wantContent:  []byte("Hello"),
			wantEncoding: EncodingUTF8,
		},
		{
			name:         "UTF-8 BOM",
			content:      append([]byte{0xEF, 0xBB, 0xBF}, []byte("Hello")...),
			wantContent:  []byte("Hello"),
			wantEncoding: EncodingUTF8BOM,
		},
		{
			name:         "UTF-16 LE BOM",
			content:      []byte{0xFF, 0xFE, 0x48, 0x00},
			wantContent:  []byte{0x48, 0x00},
			wantEncoding: EncodingUTF16LE,
		},
		{
			name:         "UTF-16 BE BOM",
			content:      []byte{0xFE, 0xFF, 0x00, 0x48},
			wantContent:  []byte{0x00, 0x48},
			wantEncoding: EncodingUTF16BE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotContent, gotEncoding := StripBOM(tt.content)
			if !bytes.Equal(gotContent, tt.wantContent) {
				t.Errorf("StripBOM() content = %v, want %v", gotContent, tt.wantContent)
			}
			if gotEncoding != tt.wantEncoding {
				t.Errorf("StripBOM() encoding = %v, want %v", gotEncoding, tt.wantEncoding)
			}
		})
	}
}

func TestAddBOM(t *testing.T) {
	content := []byte("Hello")

	tests := []struct {
		encoding Encoding
		wantBOM  []byte
	}{
		{EncodingUTF8, nil},  // No BOM added
		{EncodingASCII, nil}, // No BOM added
		{EncodingUTF8BOM, bomUTF8},
		{EncodingUTF16LE, bomUTF16LE},
		{EncodingUTF16BE, bomUTF16BE},
	}

	for _, tt := range tests {
		t.Run(string(tt.encoding), func(t *testing.T) {
			got := AddBOM(content, tt.encoding)
			if tt.wantBOM == nil {
				if !bytes.Equal(got, content) {
					t.Errorf("AddBOM() should not modify content for %v", tt.encoding)
				}
			} else {
				if !bytes.HasPrefix(got, tt.wantBOM) {
					t.Errorf("AddBOM() should add BOM for %v", tt.encoding)
				}
			}
		})
	}
}

func TestNormalizeLineEndings(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		ending  LineEnding
		want    []byte
	}{
		{
			name:    "LF to LF",
			content: []byte("a\nb\nc"),
			ending:  LineEndingLF,
			want:    []byte("a\nb\nc"),
		},
		{
			name:    "CRLF to LF",
			content: []byte("a\r\nb\r\nc"),
			ending:  LineEndingLF,
			want:    []byte("a\nb\nc"),
		},
		{
			name:    "LF to CRLF",
			content: []byte("a\nb\nc"),
			ending:  LineEndingCRLF,
			want:    []byte("a\r\nb\r\nc"),
		},
		{
			name:    "CR to LF",
			content: []byte("a\rb\rc"),
			ending:  LineEndingLF,
			want:    []byte("a\nb\nc"),
		},
		{
			name:    "mixed to CRLF",
			content: []byte("a\nb\r\nc\rd"),
			ending:  LineEndingCRLF,
			want:    []byte("a\r\nb\r\nc\r\nd"),
		},
		{
			name:    "empty",
			content: []byte{},
			ending:  LineEndingLF,
			want:    []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeLineEndings(tt.content, tt.ending)
			if !bytes.Equal(got, tt.want) {
				t.Errorf("NormalizeLineEndings() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    int
	}{
		{
			name:    "empty",
			content: []byte{},
			want:    0,
		},
		{
			name:    "single line no newline",
			content: []byte("hello"),
			want:    1,
		},
		{
			name:    "single line with LF",
			content: []byte("hello\n"),
			want:    1,
		},
		{
			name:    "two lines LF",
			content: []byte("hello\nworld"),
			want:    2,
		},
		{
			name:    "two lines CRLF",
			content: []byte("hello\r\nworld"),
			want:    2,
		},
		{
			name:    "three lines with trailing",
			content: []byte("a\nb\nc\n"),
			want:    3,
		},
		{
			name:    "mixed line endings",
			content: []byte("a\nb\r\nc\rd"),
			want:    4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CountLines(tt.content)
			if got != tt.want {
				t.Errorf("CountLines() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestIsBinary(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    bool
	}{
		{
			name:    "empty",
			content: []byte{},
			want:    false,
		},
		{
			name:    "text",
			content: []byte("Hello, World!\nThis is text."),
			want:    false,
		},
		{
			name:    "with tabs and newlines",
			content: []byte("Hello\tWorld\r\nLine 2"),
			want:    false,
		},
		{
			name:    "null byte",
			content: []byte("Hello\x00World"),
			want:    true,
		},
		{
			name:    "binary control chars",
			content: bytes.Repeat([]byte{0x01, 0x02, 0x03}, 100),
			want:    true,
		},
		{
			name:    "mostly text with few control",
			content: append(bytes.Repeat([]byte("hello world "), 100), 0x01),
			want:    false, // Less than 10%
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBinary(tt.content)
			if got != tt.want {
				t.Errorf("IsBinary() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectEncodingInfo(t *testing.T) {
	content := []byte("Hello\nWorld\n")
	info := DetectEncodingInfo(content)

	if info.Encoding != EncodingASCII {
		t.Errorf("Encoding = %v, want %v", info.Encoding, EncodingASCII)
	}
	if info.LineEnding != LineEndingLF {
		t.Errorf("LineEnding = %v, want %v", info.LineEnding, LineEndingLF)
	}
	if info.HasBOM {
		t.Error("HasBOM should be false")
	}
	if info.IsBinary {
		t.Error("IsBinary should be false")
	}
	if info.LineCount != 2 {
		t.Errorf("LineCount = %d, want 2", info.LineCount)
	}
}

func TestDetectEncodingInfo_Binary(t *testing.T) {
	content := []byte{0x00, 0x01, 0x02, 0x03}
	info := DetectEncodingInfo(content)

	if !info.IsBinary {
		t.Error("IsBinary should be true for binary content")
	}
}

func TestDetectEncodingInfo_BOM(t *testing.T) {
	content := append([]byte{0xEF, 0xBB, 0xBF}, []byte("Hello\n")...)
	info := DetectEncodingInfo(content)

	if info.Encoding != EncodingUTF8BOM {
		t.Errorf("Encoding = %v, want %v", info.Encoding, EncodingUTF8BOM)
	}
	if !info.HasBOM {
		t.Error("HasBOM should be true")
	}
}
