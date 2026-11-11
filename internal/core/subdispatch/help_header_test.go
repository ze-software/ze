package subdispatch

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestDispatchHelpHeaderSeparatesCommandAndUsage(t *testing.T) {
	for _, withDefault := range []bool{false, true} {
		d := New("install", "Native installation")
		d.Register("local", func([]string) int { return 0 }, SubMeta{Desc: "Copy the executable"})
		if withDefault {
			d.RegisterDefault(func([]string) int { t.Fatal("help ran default"); return 0 }, SubMeta{Desc: "Install this host"})
		}
		read, write, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		original := os.Stderr
		os.Stderr = write
		code := d.Dispatch([]string{"--help"})
		os.Stderr = original
		if err := write.Close(); err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(read)
		if err != nil {
			t.Fatal(err)
		}
		if err := read.Close(); err != nil {
			t.Fatal(err)
		}
		if code != 0 {
			t.Fatalf("help status=%d", code)
		}
		if strings.Contains(string(data), "[options]ze install") {
			t.Fatalf("usage leaked into header: %q", data)
		}
		if !strings.HasPrefix(string(data), "ze install - Native installation\n") {
			t.Fatalf("wrong help header: %q", data)
		}
	}
}
