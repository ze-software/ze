package setup

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// writeELF writes the smallest file debug/elf reads as a linux executable for
// machine: a 64-bit little-endian header with no section or program header.
func writeELF(t *testing.T, machine elf.Machine) string {
	t.Helper()
	header := elf.Header64{
		Type:      uint16(elf.ET_EXEC),
		Machine:   uint16(machine),
		Version:   uint32(elf.EV_CURRENT),
		Ehsize:    64,
		Phentsize: 56,
		Shentsize: 64,
	}
	copy(header.Ident[:], elf.ELFMAG)
	header.Ident[elf.EI_CLASS] = byte(elf.ELFCLASS64)
	header.Ident[elf.EI_DATA] = byte(elf.ELFDATA2LSB)
	header.Ident[elf.EI_VERSION] = byte(elf.EV_CURRENT)
	var file bytes.Buffer
	if err := binary.Write(&file, binary.LittleEndian, &header); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "ze")
	if err := os.WriteFile(path, file.Bytes(), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// refuseBuild fails the test if the check builds a ze it was handed.
func refuseBuild(t *testing.T) func(string) (string, error) {
	t.Helper()
	return func(goarch string) (string, error) {
		t.Errorf("built a linux ze for %s although the operator named one", goarch)
		return "", nil
	}
}

// VALIDATES: defect 1 of the owner's 2026-10-10 Linux run. With no `ze` the
// check builds the linux ze for the DAEMON's architecture through the one
// producer docker-lab uses, and mounts the file that producer wrote.
// PREVENTS: the repository root mounted as /ze, which the container cannot
// execute (exit 127), and a probe built for the client's architecture.
func TestDockerKernelProbeZeBuildsForTheDaemonArchitecture(t *testing.T) {
	built := writeELF(t, elf.EM_AARCH64)
	var asked []string
	ze, err := dockerKernelProbeZe("", "arm64", func(goarch string) (string, error) {
		asked = append(asked, goarch)
		return built, nil
	})
	if err != nil {
		t.Fatalf("the built arm64 ze was refused: %v", err)
	}
	if ze != built {
		t.Errorf("mounts %q, want the built %q", ze, built)
	}
	if len(asked) != 1 || asked[0] != "arm64" {
		t.Errorf("built for %v, want once for the daemon's arm64", asked)
	}
}

// VALIDATES: a ze the operator names is used as named when it is a linux
// executable for the daemon's architecture, and nothing is built.
func TestDockerKernelProbeZeUsesTheNamedBinary(t *testing.T) {
	named := writeELF(t, elf.EM_X86_64)
	ze, err := dockerKernelProbeZe(named, "amd64", refuseBuild(t))
	if err != nil {
		t.Fatalf("a matching amd64 ze was refused: %v", err)
	}
	if ze != named {
		t.Errorf("mounts %q, want %q", ze, named)
	}
}

// VALIDATES: whatever reaches the mount, named or built, is a linux executable
// for the daemon's architecture, or the check refuses before any container
// runs, naming the path, what it is, and the route that builds the right one.
// PREVENTS: a directory, a darwin build or another architecture's ze mounted
// as /ze, whose only symptom is exit 127 or "exec format error".
func TestDockerKernelProbeZeRefusesAnythingButTheDaemonsLinuxBinary(t *testing.T) {
	text := filepath.Join(t.TempDir(), "notes")
	if err := os.WriteFile(text, []byte("not a program\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	amd64 := writeELF(t, elf.EM_X86_64)
	for name, test := range map[string]struct {
		named, goarch string
		wants         []string
	}{
		"directory":    {directory, "amd64", []string{directory, "directory"}},
		"not ELF":      {text, "amd64", []string{text, "ELF"}},
		"other arch":   {amd64, "arm64", []string{amd64, "EM_X86_64", "arm64"}},
		"unknown arch": {amd64, "pdp11", []string{"pdp11"}},
		"missing":      {filepath.Join(directory, "absent"), "amd64", []string{"absent"}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := dockerKernelProbeZe(test.named, test.goarch, refuseBuild(t))
			if err == nil {
				t.Fatalf("%s was mounted for a %s daemon", test.named, test.goarch)
			}
			for _, want := range append(test.wants, "omit `ze`") {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal does not name %q: %v", want, err)
				}
			}
		})
	}

	// The built file is judged too: the check pairs the producer with a reader.
	if _, err := dockerKernelProbeZe("", "arm64", func(string) (string, error) { return amd64, nil }); err == nil {
		t.Error("a built ze of the wrong architecture was mounted")
	}
}

// VALIDATES: `ze` is optional on check, so the owner's bare
// `./le setup docker-kernel check` resolves the binary rather than refusing.
func TestDockerKernelCheckZeIsOptional(t *testing.T) {
	for _, action := range DockerKernelActions().Actions {
		if action.Verb != dockerKernelCheckVerb {
			continue
		}
		for _, parameter := range action.Parameters {
			if parameter.Keyword == dockerKernelZeKeyword && parameter.Requirement != leaction.Optional {
				t.Errorf("check's %q is not optional", dockerKernelZeKeyword)
			}
		}
		return
	}
	t.Fatalf("no %s action", dockerKernelCheckVerb)
}
