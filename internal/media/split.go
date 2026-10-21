package media

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// DefaultPartSize is the size of a single part when a file is split and no
// other size is known.
//
// The server refuses every file part at or beyond 2000 MB, the parts stay
// below that limit with a small margin.
const DefaultPartSize int64 = 1900 * 1024 * 1024

// Part describes one part of a split file.
type Part struct {
	Index int
	Path  string
	Name  string
	Size  int64
}

// SplitResult describes the parts of a split file.
type SplitResult struct {
	Original string
	Parts    []Part
}

// SplitFile splits a file into parts of at most partSize bytes.
//
// The parts are written into their own directory and are named after the
// original file, so they can be joined again in a shell with a simple glob.
func SplitFile(path string, partSize int64) (SplitResult, error) {
	result := SplitResult{Original: path}
	if partSize <= 0 {
		partSize = DefaultPartSize
	}

	info, err := os.Stat(path)
	if err != nil {
		return result, err
	}
	if info.Size() <= partSize {
		return result, fmt.Errorf("the file is smaller than a single part")
	}

	directory, err := os.MkdirTemp(filepath.Dir(path), "iota-split-")
	if err != nil {
		return result, err
	}

	base := filepath.Base(path)
	source, err := os.Open(path)
	if err != nil {
		_ = os.RemoveAll(directory)
		return result, err
	}
	defer func() { _ = source.Close() }()

	count := int((info.Size() + partSize - 1) / partSize)
	width := len(fmt.Sprintf("%d", count))
	if width < 2 {
		width = 2
	}

	buffer := make([]byte, 1<<20)
	for index := 0; index < count; index++ {
		name := fmt.Sprintf("%s.part%0*d", base, width, index+1)
		target := filepath.Join(directory, name)

		part, createErr := os.Create(target)
		if createErr != nil {
			_ = os.RemoveAll(directory)
			return result, createErr
		}

		written, copyErr := copyPart(source, part, partSize, buffer)
		closeErr := part.Close()
		if copyErr != nil {
			_ = os.RemoveAll(directory)
			return result, copyErr
		}
		if closeErr != nil {
			_ = os.RemoveAll(directory)
			return result, closeErr
		}

		result.Parts = append(result.Parts, Part{Index: index + 1, Path: target, Name: name, Size: written})
	}

	return result, nil
}

// copyPart copies at most limit bytes from the source into the target.
func copyPart(source io.Reader, target io.Writer, limit int64, buffer []byte) (int64, error) {
	var written int64
	for written < limit {
		chunk := int64(len(buffer))
		if remaining := limit - written; remaining < chunk {
			chunk = remaining
		}
		read, err := source.Read(buffer[:chunk])
		if read > 0 {
			if _, writeErr := target.Write(buffer[:read]); writeErr != nil {
				return written, writeErr
			}
			written += int64(read)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return written, err
		}
	}
	return written, nil
}

// RemoveParts deletes the parts of a split file.
func RemoveParts(result SplitResult) {
	if len(result.Parts) == 0 {
		return
	}
	for _, part := range result.Parts {
		_ = os.Remove(part.Path)
	}
	directory := filepath.Dir(result.Parts[0].Path)
	if directory != "" {
		_ = os.RemoveAll(directory)
	}
}

// JoinCommand renders the shell command that joins the parts again.
func JoinCommand(result SplitResult) string {
	if len(result.Parts) == 0 {
		return ""
	}
	pattern := strings.TrimRight(result.Parts[0].Name, "0123456789")
	return fmt.Sprintf("cat %s* > %s", pattern, filepath.Base(result.Original))
}
