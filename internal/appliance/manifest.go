// Design: docs/architecture/appliance/builder.md -- build manifest and image checksums

package appliance

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

type BuildManifest struct {
	Appliance   string `json:"appliance"`
	Timestamp   string `json:"timestamp"`
	ZeVersion   string `json:"ze-version"`
	Arch        string `json:"arch"`
	ConfigHash  string `json:"config-hash"`
	Image       string `json:"image"`
	ImageSHA256 string `json:"image-sha256"`
}

func WriteManifest(path string, m *BuildManifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644) //nolint:gosec // informational file
}

func ImageTimestamp() string {
	return time.Now().Format("20060102-150405")
}

func ImageFileName(ts string) string {
	return fmt.Sprintf("ze-%s.img", ts)
}

func WriteImageChecksum(imagePath, checksumPath string) (string, error) {
	f, err := os.Open(imagePath) //nolint:gosec // user-provided path
	if err != nil {
		return "", fmt.Errorf("open image: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash image: %w", err)
	}

	sum := fmt.Sprintf("%x", h.Sum(nil))
	line := fmt.Sprintf("%s  %s\n", sum, imagePath)

	if err := os.WriteFile(checksumPath, []byte(line), 0o644); err != nil { //nolint:gosec // informational
		return "", fmt.Errorf("write checksum: %w", err)
	}
	return sum, nil
}

func ConfigHash(seedConfig string) string {
	h := sha256.Sum256([]byte(seedConfig))
	return fmt.Sprintf("sha256:%x", h)
}
