package rope

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// generateText creates a string of the given size with realistic content.
func generateText(size int) string {
	var sb strings.Builder
	sb.Grow(size)

	words := []string{"the", "quick", "brown", "fox", "jumps", "over", "lazy", "dog", "hello", "world"}
	lineLen := 0

	for sb.Len() < size {
		word := words[rand.Intn(len(words))]
		if sb.Len()+len(word)+1 > size {
			break
		}

		if sb.Len() > 0 {
			if lineLen > 60 {
				sb.WriteByte('\n')
				lineLen = 0
			} else {
				sb.WriteByte(' ')
				lineLen++
			}
		}

		sb.WriteString(word)
		lineLen += len(word)
	}

	return sb.String()
}

// generateTextWithLines creates text with approximately the given number of lines.
func generateTextWithLines(lines int, avgLineLen int) string {
	var sb strings.Builder
	sb.Grow(lines * (avgLineLen + 1))

	for i := 0; i < lines; i++ {
		lineLen := avgLineLen + rand.Intn(21) - 10 // +/- 10
		if lineLen < 10 {
			lineLen = 10
		}
		for j := 0; j < lineLen; j++ {
			sb.WriteByte(byte('a' + rand.Intn(26)))
		}
		if i < lines-1 {
			sb.WriteByte('\n')
		}
	}

	return sb.String()
}

// Benchmarks for rope creation

func BenchmarkFromString(b *testing.B) {
	sizes := []int{100, 1000, 10000, 100000, 1000000}

	for _, size := range sizes {
		text := generateText(size)
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = FromString(text)
			}
		})
	}
}

func BenchmarkBuilder(b *testing.B) {
	sizes := []int{100, 1000, 10000, 100000}
	chunkSize := 100

	for _, size := range sizes {
		text := generateText(size)
		chunks := make([]string, 0, size/chunkSize+1)
		for i := 0; i < len(text); i += chunkSize {
			end := i + chunkSize
			if end > len(text) {
				end = len(text)
			}
			chunks = append(chunks, text[i:end])
		}

		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				builder := NewBuilder()
				for _, chunk := range chunks {
					builder.WriteString(chunk)
				}
				_ = builder.Build()
			}
		})
	}
}

// Benchmarks for insert operations

func BenchmarkInsertStart(b *testing.B) {
	sizes := []int{1000, 10000, 100000}

	for _, size := range sizes {
		text := generateText(size)
		r := FromString(text)

		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = r.Insert(0, "x")
			}
		})
	}
}

func BenchmarkInsertMiddle(b *testing.B) {
	sizes := []int{1000, 10000, 100000}

	for _, size := range sizes {
		text := generateText(size)
		r := FromString(text)
		mid := ByteOffset(size / 2)

		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = r.Insert(mid, "x")
			}
		})
	}
}

func BenchmarkInsertEnd(b *testing.B) {
	sizes := []int{1000, 10000, 100000}

	for _, size := range sizes {
		text := generateText(size)
		r := FromString(text)
		end := ByteOffset(size)

		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = r.Insert(end, "x")
			}
		})
	}
}

func BenchmarkInsertRandom(b *testing.B) {
	sizes := []int{1000, 10000, 100000}

	for _, size := range sizes {
		text := generateText(size)
		r := FromString(text)

		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				offset := ByteOffset(rand.Intn(size))
				_ = r.Insert(offset, "x")
			}
		})
	}
}

// Benchmarks for delete operations

func BenchmarkDeleteMiddle(b *testing.B) {
	sizes := []int{1000, 10000, 100000}

	for _, size := range sizes {
		text := generateText(size)
		r := FromString(text)
		start := ByteOffset(size/2 - 50)
		end := ByteOffset(size/2 + 50)

		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = r.Delete(start, end)
			}
		})
	}
}

// Benchmarks for concatenation

func BenchmarkConcat(b *testing.B) {
	sizes := []int{1000, 10000, 100000}

	for _, size := range sizes {
		text1 := generateText(size / 2)
		text2 := generateText(size / 2)
		r1 := FromString(text1)
		r2 := FromString(text2)

		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = r1.Concat(r2)
			}
		})
	}
}

// Benchmarks for split

func BenchmarkSplit(b *testing.B) {
	sizes := []int{1000, 10000, 100000}

	for _, size := range sizes {
		text := generateText(size)
		r := FromString(text)
		mid := ByteOffset(size / 2)

		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = r.Split(mid)
			}
		})
	}
}

// Benchmarks for access operations

func BenchmarkByteAt(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}

	for _, size := range sizes {
		text := generateText(size)
		r := FromString(text)

		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				offset := ByteOffset(rand.Intn(size))
				_, _ = r.ByteAt(offset)
			}
		})
	}
}

func BenchmarkSlice(b *testing.B) {
	sizes := []int{1000, 10000, 100000}

	for _, size := range sizes {
		text := generateText(size)
		r := FromString(text)

		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := ByteOffset(rand.Intn(size - 100))
				end := start + 100
				_ = r.Slice(start, end)
			}
		})
	}
}

// Benchmarks for line operations

func BenchmarkLineCount(b *testing.B) {
	lineCounts := []int{100, 1000, 10000}

	for _, lines := range lineCounts {
		text := generateTextWithLines(lines, 80)
		r := FromString(text)

		b.Run(fmt.Sprintf("lines=%d", lines), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = r.LineCount()
			}
		})
	}
}

func BenchmarkLineText(b *testing.B) {
	lineCounts := []int{100, 1000, 10000}

	for _, lines := range lineCounts {
		text := generateTextWithLines(lines, 80)
		r := FromString(text)

		b.Run(fmt.Sprintf("lines=%d", lines), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				line := uint32(rand.Intn(lines))
				_ = r.LineText(line)
			}
		})
	}
}

func BenchmarkLineStartOffset(b *testing.B) {
	lineCounts := []int{100, 1000, 10000}

	for _, lines := range lineCounts {
		text := generateTextWithLines(lines, 80)
		r := FromString(text)

		b.Run(fmt.Sprintf("lines=%d", lines), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				line := uint32(rand.Intn(lines))
				_ = r.LineStartOffset(line)
			}
		})
	}
}

// Benchmarks for coordinate conversion

func BenchmarkOffsetToPoint(b *testing.B) {
	lineCounts := []int{100, 1000, 10000}

	for _, lines := range lineCounts {
		text := generateTextWithLines(lines, 80)
		r := FromString(text)
		size := len(text)

		b.Run(fmt.Sprintf("lines=%d", lines), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				offset := ByteOffset(rand.Intn(size))
				_ = r.OffsetToPoint(offset)
			}
		})
	}
}

func BenchmarkPointToOffset(b *testing.B) {
	lineCounts := []int{100, 1000, 10000}

	for _, lines := range lineCounts {
		text := generateTextWithLines(lines, 80)
		r := FromString(text)

		b.Run(fmt.Sprintf("lines=%d", lines), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				point := Point{
					Line:   uint32(rand.Intn(lines)),
					Column: uint32(rand.Intn(80)),
				}
				_ = r.PointToOffset(point)
			}
		})
	}
}

// Benchmarks for cursor operations

func BenchmarkCursorSeekOffset(b *testing.B) {
	sizes := []int{1000, 10000, 100000}

	for _, size := range sizes {
		text := generateText(size)
		r := FromString(text)

		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			cursor := NewCursor(r)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				offset := ByteOffset(rand.Intn(size))
				cursor.SeekOffset(offset)
			}
		})
	}
}

func BenchmarkCursorSeekLine(b *testing.B) {
	lineCounts := []int{100, 1000, 10000}

	for _, lines := range lineCounts {
		text := generateTextWithLines(lines, 80)
		r := FromString(text)

		b.Run(fmt.Sprintf("lines=%d", lines), func(b *testing.B) {
			cursor := NewCursor(r)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				line := uint32(rand.Intn(lines))
				cursor.SeekLine(line)
			}
		})
	}
}

func BenchmarkCursorIterate(b *testing.B) {
	sizes := []int{1000, 10000}

	for _, size := range sizes {
		text := generateText(size)
		r := FromString(text)

		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				cursor := NewCursor(r)
				for cursor.Next() {
				}
			}
		})
	}
}

// Benchmarks for iterators

func BenchmarkChunkIterator(b *testing.B) {
	sizes := []int{1000, 10000, 100000}

	for _, size := range sizes {
		text := generateText(size)
		r := FromString(text)

		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				iter := r.Chunks()
				for iter.Next() {
					_ = iter.Chunk()
				}
			}
		})
	}
}

func BenchmarkLineIterator(b *testing.B) {
	lineCounts := []int{100, 1000, 10000}

	for _, lines := range lineCounts {
		text := generateTextWithLines(lines, 80)
		r := FromString(text)

		b.Run(fmt.Sprintf("lines=%d", lines), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				iter := r.Lines()
				for iter.Next() {
					_ = iter.Text()
				}
			}
		})
	}
}

// Benchmark comparing to string operations

func BenchmarkStringVsRopeInsert(b *testing.B) {
	sizes := []int{1000, 10000}

	for _, size := range sizes {
		text := generateText(size)

		b.Run(fmt.Sprintf("string_size=%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				mid := size / 2
				_ = text[:mid] + "x" + text[mid:]
			}
		})

		r := FromString(text)
		b.Run(fmt.Sprintf("rope_size=%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = r.Insert(ByteOffset(size/2), "x")
			}
		})
	}
}
