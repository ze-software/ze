package packer

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/gokrazy/internal/config"
	"github.com/gokrazy/internal/fat"
)

// TestPackedCmdlineSupportsTestboot reads the actual packed FAT files. Both
// command-line formats need their newline delimiter and reserved update space;
// the runtime's retained-extent tests separately prove safe in-place mutation.
func TestPackedCmdlineSupportsTestboot(t *testing.T) {
	source := filepath.Join(t.TempDir(), "cmdline.txt")
	if err := os.WriteFile(source, []byte("root=/dev/sda2 quiet\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var image bytes.Buffer
	writer, err := fat.NewWriter(&image)
	if err != nil {
		t.Fatal(err)
	}
	pack := &Pack{Cfg: &config.Struct{SerialConsole: "disabled"}, Env: Osenv{Stdout: io.Discard, Stderr: io.Discard}}
	pack.UseGPTPartuuid = true
	if err := pack.writeCmdline(writer, source); err != nil {
		t.Fatal(err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
	reader, err := fat.NewReader(bytes.NewReader(image.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/cmdline.txt", "/loader/entries/gokrazy.conf"} {
		t.Run(path, func(t *testing.T) {
			offset, length, err := reader.Extents(path)
			if err != nil {
				t.Fatal(err)
			}
			data := image.Bytes()[offset : offset+length]
			if !bytes.HasSuffix(data, []byte{'\n'}) {
				t.Fatalf("packed %s has no trailing newline for the runtime testboot edit", path)
			}
			if !bytes.Contains(data, bytes.Repeat([]byte{' '}, 64)) {
				t.Fatalf("packed %s lost the reserved command-line space", path)
			}
			// Exercise the newline append position on both packed formats. This
			// checks the file delimiter, not the runtime's retained-extent update.
			marked := bytes.ReplaceAll(data, []byte{'\n'}, []byte(" gokrazy.try_boot_inactive=1\n"))
			line := bytes.TrimSpace(marked)
			if path != "/cmdline.txt" {
				at := bytes.LastIndex(line, []byte("options "))
				if at < 0 {
					t.Fatal("packed loader entry lost its options line")
				}
				line = line[at:]
			}
			if !bytes.Contains(line, []byte("gokrazy.try_boot_inactive=1")) {
				t.Fatalf("newline insertion did not reach %s's kernel arguments", path)
			}
		})
	}
}
