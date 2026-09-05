package qemu

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: the run refuses a binary the musl guest cannot exec, before it
// spends an hour proving it one test at a time.
// PREVENTS: the 2026-09-04 run, where a `bin/ze` built without CGO_ENABLED=0
// named the glibc loader, the guest had no such loader, and 326 `.ci` tests
// each reported `start ze: fork/exec ...: no such file or directory`. verify()
// stat'd the file, saw a regular executable, and started the run.

// elfHeaderSize and elfProgSize are the ELF64 header and program-header sizes
// the fixture writes. debug/elf reads the program header table at Phoff using
// the sizes the header declares, so the two must agree with what is written.
const (
	elfHeaderSize = 64
	elfProgSize   = 56
)

// staticELF builds the bytes of a minimal ELF64 executable that names no
// dynamic loader, which is the shape `go build` with CGO_ENABLED=0 produces.
func staticELF() []byte {
	var out bytes.Buffer
	writeELFHeader(&out, 0, 0)
	return out.Bytes()
}

// dynamicELF builds the bytes of a minimal ELF64 executable carrying one
// PT_INTERP program header that names loader, which is the shape a cgo build on
// a glibc host produces.
func dynamicELF(loader string) []byte {
	var out bytes.Buffer
	writeELFHeader(&out, elfHeaderSize, 1)

	interp := append([]byte(loader), 0)
	prog := elf.Prog64{
		Type:   uint32(elf.PT_INTERP),
		Flags:  uint32(elf.PF_R),
		Off:    elfHeaderSize + elfProgSize,
		Filesz: uint64(len(interp)),
		Memsz:  uint64(len(interp)),
		Align:  1,
	}
	binary.Write(&out, binary.LittleEndian, prog) //nolint:errcheck // an in-memory buffer
	out.Write(interp)                             //nolint:errcheck // an in-memory buffer
	return out.Bytes()
}

// writeELFHeader writes the 64-byte ELF64 header. A file with no section header
// table declares Shoff 0 and Shnum 0, which debug/elf accepts.
func writeELFHeader(out *bytes.Buffer, phoff uint64, phnum uint16) {
	header := elf.Header64{
		Type:      uint16(elf.ET_EXEC),
		Machine:   uint16(elf.EM_X86_64),
		Version:   uint32(elf.EV_CURRENT),
		Phoff:     phoff,
		Ehsize:    elfHeaderSize,
		Phentsize: elfProgSize,
		Phnum:     phnum,
	}
	copy(header.Ident[:], elf.ELFMAG)
	header.Ident[elf.EI_CLASS] = byte(elf.ELFCLASS64)
	header.Ident[elf.EI_DATA] = byte(elf.ELFDATA2LSB)
	header.Ident[elf.EI_VERSION] = byte(elf.EV_CURRENT)
	binary.Write(out, binary.LittleEndian, header) //nolint:errcheck // an in-memory buffer
}

// writeBinary puts one built binary in the workspace, executable.
func writeBinary(t *testing.T, root, rel string, body []byte) string {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, body, 0o700); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
	return full
}

// The fixture the whole suite runs on is itself an ELF the guest could exec.
// Without this, every other case in this package would pass against a binary
// the VM cannot start.
func TestAStaticBinaryIsAccepted(t *testing.T) {
	path := writeBinary(t, t.TempDir(), "bin/ze", staticELF())
	if err := runnableInGuest(path); err != nil {
		t.Fatalf("a statically linked ELF was refused: %v", err)
	}
}

// The defect this check exists for: the loader is named in the file and the
// stat says nothing about it.
func TestADynamicallyLinkedBinaryIsRefusedBeforeAnythingRuns(t *testing.T) {
	const loader = "/lib64/ld-linux-x86-64.so.2"
	path := writeBinary(t, t.TempDir(), "bin/ze", dynamicELF(loader))

	err := runnableInGuest(path)
	if err == nil {
		t.Fatal("a glibc-dynamic binary was accepted, so the run would report the loader's" +
			" ENOENT once for every test in every suite")
	}
	if !strings.Contains(err.Error(), loader) {
		t.Errorf("the refusal is %q, want it to name the loader %q", err, loader)
	}
	if !strings.Contains(err.Error(), "CGO_ENABLED=0") {
		t.Errorf("the refusal is %q, want it to name the fix", err)
	}
}

// A darwin binary, or a shell script, reaches the same refusal. The guest can
// exec none of them, and each one used to read as a product failure.
func TestABinaryThatIsNotALinuxELFIsRefused(t *testing.T) {
	path := writeBinary(t, t.TempDir(), "bin/ze", []byte("#!/bin/sh\nexit 0\n"))
	err := runnableInGuest(path)
	if err == nil {
		t.Fatal("a file that is not an ELF binary was accepted as one of the run's three binaries")
	}
	if !strings.Contains(err.Error(), "ELF") {
		t.Errorf("the refusal is %q, want it to say the file is not a Linux ELF binary", err)
	}
}

// The refusal happens in verify, before any child, so an hour of VM time is
// not spent on a run that cannot answer.
func TestADynamicBinaryStopsTheRunBeforeAnyChild(t *testing.T) {
	run := vmFixture(t)
	writeBinary(t, run.Workspace, "bin/ze", dynamicELF("/lib64/ld-linux-x86-64.so.2"))
	rec := &recorder{}
	run.Run = rec.run

	report, code := run.Execute()
	if code == 0 {
		t.Fatal("a run whose ze cannot exec in the guest exited 0")
	}
	if len(rec.calls) != 0 {
		t.Errorf("%d children ran before the refusal, want 0", len(rec.calls))
	}
	if len(report.Phases) != 0 {
		t.Errorf("the report carries %d phases, want none", len(report.Phases))
	}
}

// A PT_INTERP header whose declared length the file does not carry is still a
// refusal. The binary names a loader either way, so answering nil on a short
// read would accept exactly the file this check exists to stop.
//
// The bound itself (loaderNameMax) is not observable from here: without it the
// same case allocates four gibibytes and reaches the same refusal.
func TestABinaryWhoseLoaderDeclarationIsUnreadableIsStillRefused(t *testing.T) {
	body := dynamicELF(strings.Repeat("a", 64))
	// Re-declare the PT_INTERP length as 4 GiB, leaving the file itself short.
	binary.LittleEndian.PutUint64(body[elfHeaderSize+32:], 1<<32)

	err := runnableInGuest(writeBinary(t, t.TempDir(), "bin/ze", body))
	if err == nil {
		t.Fatal("a binary declaring a dynamic loader was accepted")
	}
	if !strings.Contains(err.Error(), "CGO_ENABLED=0") {
		t.Errorf("the refusal is %q, want it to name the fix", err)
	}
}
