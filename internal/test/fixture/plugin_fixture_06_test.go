package fixture

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFixture06FrameCheckAcceptsEveryContractCase(t *testing.T) {
	t.Parallel()
	streamedBody := strings.Repeat("{}\n", 257)
	failedMessage := "pki: certificate Жé not found"
	tests := []struct {
		name  string
		frame string
		body  string
	}{
		{name: "document", frame: "top doc 0: 0:\nend 1 0 0:\n", body: "version\n"},
		{name: "streamed", frame: "top map 8:commands 0:\nend 257 0 0:\n", body: streamedBody},
		{name: "unknown", frame: "nay 0: 15:unknown command\n"},
		{name: "failed", frame: fmt.Sprintf("top doc 0: 0:\nend 0 0 %d:%s\n", len(failedMessage), failedMessage)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			framePath := filepath.Join(dir, "frame")
			if err := os.WriteFile(framePath, []byte(test.frame), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{test.name, framePath}
			if test.body != "" || test.name == "unknown" {
				bodyPath := filepath.Join(dir, "body")
				if err := os.WriteFile(bodyPath, []byte(test.body), 0o600); err != nil {
					t.Fatal(err)
				}
				args = append(args, bodyPath)
			}
			if err := fixture06FrameCheck(context.Background(), args); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFixture06FrameCheckRejectsCRLF(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	framePath := filepath.Join(dir, "frame")
	if err := os.WriteFile(framePath, []byte("top doc 0: 0:\r\nend 1 0 0:\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := fixture06FrameCheck(context.Background(), []string{"document", framePath}); err == nil {
		t.Fatal("CRLF frame accepted")
	}
}
