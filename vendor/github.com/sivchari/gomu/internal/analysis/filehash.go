// Package analysis provides file analysis and hashing functionality.
package analysis

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// FileHasher provides file hashing functionality.
type FileHasher struct{}

// NewFileHasher creates a new file hasher.
func NewFileHasher() *FileHasher {
	return &FileHasher{}
}

// HashFile calculates the SHA256 hash of a file.
func (h *FileHasher) HashFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to hash file %s: %w", filePath, err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// HashFiles calculates hashes for multiple files.
func (h *FileHasher) HashFiles(filePaths []string) (map[string]string, error) {
	hashes := make(map[string]string)

	for _, filePath := range filePaths {
		hash, err := h.HashFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to hash file %s: %w", filePath, err)
		}

		hashes[filePath] = hash
	}

	return hashes, nil
}

// HashContent calculates the SHA256 hash of content.
func (h *FileHasher) HashContent(content []byte) string {
	hash := sha256.New()
	hash.Write(content)

	return fmt.Sprintf("%x", hash.Sum(nil))
}
