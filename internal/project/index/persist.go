package index

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Persistence format version
const persistVersion = 1

// Magic bytes for file identification
var persistMagic = []byte("KSIX") // KeyStorm IndeX

// Persistence errors
var (
	ErrInvalidFormat   = errors.New("invalid index format")
	ErrVersionMismatch = errors.New("index version mismatch")
)

// Maximum string length in persistence format (16 MB should be more than enough for any path)
const maxStringLength = 16 * 1024 * 1024

// Save persists the index to a writer.
// Format:
//
//	[4 bytes] Magic "KSIX"
//	[4 bytes] Version (little endian)
//	[4 bytes] Entry count (little endian)
//	[entries...]
//	  [4 bytes] Path length
//	  [n bytes] Path
//	  [4 bytes] Name length
//	  [n bytes] Name
//	  [8 bytes] Size (little endian)
//	  [8 bytes] ModTime (Unix nano, little endian)
//	  [1 byte]  Flags (IsDir, IsSymlink)
//	  [4 bytes] Mode (little endian)
func (fi *FileIndex) Save(w io.Writer) error {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	if fi.closed {
		return ErrIndexClosed
	}

	bw := bufio.NewWriter(w)

	// Write magic
	if _, err := bw.Write(persistMagic); err != nil {
		return err
	}

	// Write version
	if err := binary.Write(bw, binary.LittleEndian, uint32(persistVersion)); err != nil {
		return err
	}

	// Write entry count
	if err := binary.Write(bw, binary.LittleEndian, uint32(len(fi.entries))); err != nil {
		return err
	}

	// Write entries
	for path, info := range fi.entries {
		if err := writeEntry(bw, path, info); err != nil {
			return err
		}
	}

	return bw.Flush()
}

// Load restores the index from a reader.
func (fi *FileIndex) Load(r io.Reader) error {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	if fi.closed {
		return ErrIndexClosed
	}

	br := bufio.NewReader(r)

	// Read and verify magic
	magic := make([]byte, 4)
	if _, err := io.ReadFull(br, magic); err != nil {
		return err
	}
	if string(magic) != string(persistMagic) {
		return ErrInvalidFormat
	}

	// Read and verify version
	var version uint32
	if err := binary.Read(br, binary.LittleEndian, &version); err != nil {
		return err
	}
	if version != persistVersion {
		return ErrVersionMismatch
	}

	// Read entry count
	var count uint32
	if err := binary.Read(br, binary.LittleEndian, &count); err != nil {
		return err
	}

	// Clear existing entries
	fi.entries = make(map[string]FileInfo, count)
	fi.nameIndex = make(map[string][]string)
	fi.dirIndex = make(map[string][]string)

	// Read entries
	for i := uint32(0); i < count; i++ {
		path, info, err := readEntry(br)
		if err != nil {
			return err
		}

		fi.entries[path] = info

		// Rebuild indexes (must match Add() behavior)
		nameLower := strings.ToLower(info.Name)
		fi.nameIndex[nameLower] = append(fi.nameIndex[nameLower], path)

		dir := filepath.Dir(path)
		fi.dirIndex[dir] = append(fi.dirIndex[dir], path)
	}

	return nil
}

func writeEntry(w *bufio.Writer, path string, info FileInfo) error {
	// Path
	if err := writeString(w, path); err != nil {
		return err
	}

	// Name
	if err := writeString(w, info.Name); err != nil {
		return err
	}

	// Size
	if err := binary.Write(w, binary.LittleEndian, info.Size); err != nil {
		return err
	}

	// ModTime (Unix nano)
	if err := binary.Write(w, binary.LittleEndian, info.ModTime.UnixNano()); err != nil {
		return err
	}

	// Flags
	var flags byte
	if info.IsDir {
		flags |= 0x01
	}
	if info.IsSymlink {
		flags |= 0x02
	}
	if err := w.WriteByte(flags); err != nil {
		return err
	}

	// Mode
	if err := binary.Write(w, binary.LittleEndian, uint32(info.Mode)); err != nil {
		return err
	}

	return nil
}

func readEntry(r *bufio.Reader) (string, FileInfo, error) {
	var info FileInfo

	// Path
	path, err := readString(r)
	if err != nil {
		return "", info, err
	}

	// Name
	info.Name, err = readString(r)
	if err != nil {
		return "", info, err
	}

	// Size
	if err := binary.Read(r, binary.LittleEndian, &info.Size); err != nil {
		return "", info, err
	}

	// ModTime
	var modTimeNano int64
	if err := binary.Read(r, binary.LittleEndian, &modTimeNano); err != nil {
		return "", info, err
	}
	info.ModTime = time.Unix(0, modTimeNano)

	// Flags
	flags, err := r.ReadByte()
	if err != nil {
		return "", info, err
	}
	info.IsDir = flags&0x01 != 0
	info.IsSymlink = flags&0x02 != 0

	// Mode
	var mode uint32
	if err := binary.Read(r, binary.LittleEndian, &mode); err != nil {
		return "", info, err
	}
	info.Mode = os.FileMode(mode)

	info.Path = path
	return path, info, nil
}

func writeString(w *bufio.Writer, s string) error {
	if len(s) > maxStringLength {
		return ErrInvalidFormat
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(len(s))); err != nil {
		return err
	}
	_, err := w.WriteString(s)
	return err
}

func readString(r *bufio.Reader) (string, error) {
	var length uint32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return "", err
	}

	// Validate length to prevent OOM attacks from malformed files
	if length > maxStringLength {
		return "", ErrInvalidFormat
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}

	return string(buf), nil
}

// SaveToFile saves the index to a file.
func (fi *FileIndex) SaveToFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return fi.Save(f)
}

// LoadFromFile loads the index from a file.
func (fi *FileIndex) LoadFromFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return fi.Load(f)
}
