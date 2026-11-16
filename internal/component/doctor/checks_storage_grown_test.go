// Design: docs/architecture/doctor-and-health-checks.md -- persistent storage smaller than its disk.

package doctor

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
)

// persistentStorageAnswers replaces the read of the persistent storage with a
// fixed answer for one test, and puts the host component's read back after it.
func persistentStorageAnswers(t *testing.T, storage host.PersistentStorage, err error) {
	t.Helper()
	hostRead := readPersistentStorage
	readPersistentStorage = func() (host.PersistentStorage, error) { return storage, err }
	t.Cleanup(func() { readPersistentStorage = hostRead })
}

// TestStorageDoctorReportsStorageSmallerThanDisk asks what the storage-grown
// check reports for each answer the host component can give about the
// persistent storage. The read is a seam, so every answer is given directly.
//
// VALIDATES: storage smaller than its disk is one warning under
// doctor-storage-not-grown that names the device and both sizes, with the
// command that grows it as its fix; grown storage and a system with no
// persistent storage report nothing; a state that cannot be read is a warning
// with its cause, never the healthy empty answer.
// PREVENTS: an unread state passing for grown storage, and a finding the
// operator cannot act on because it names no command.
func TestStorageDoctorReportsStorageSmallerThanDisk(t *testing.T) {
	const (
		imageBytes = 944 << 20
		diskBytes  = 238 << 30
	)

	t.Run("smaller than the disk", func(t *testing.T) {
		persistentStorageAnswers(t, host.PersistentStorage{
			Device:          "/dev/sda4",
			DiskBytes:       diskBytes,
			PartitionBytes:  imageBytes,
			FilesystemBytes: imageBytes,
		}, nil)
		diags := checkStorageGrown()
		if len(diags) != 1 {
			t.Fatalf("diagnostics = %+v, want one", diags)
		}
		got := diags[0]
		if got.Code != diagnostic.CodeDoctorStorageNotGrown || got.Severity != diagnostic.SeverityWarning {
			t.Errorf("finding is %s %s, want a %s warning", got.Severity, got.Code, diagnostic.CodeDoctorStorageNotGrown)
		}
		const message = "persistent storage /dev/sda4 uses 944 MiB of a 238 GiB disk, " +
			"because it was not grown to the disk when this system was installed"
		if got.Message != message {
			t.Errorf("message = %q, want %q", got.Message, message)
		}
		if got.Fix != "request system storage grow" {
			t.Errorf("fix = %q, want the command that grows the storage", got.Fix)
		}
		if got.Path != "/dev/sda4" {
			t.Errorf("path = %q, want the device", got.Path)
		}
	})

	t.Run("partition grown and filesystem not", func(t *testing.T) {
		persistentStorageAnswers(t, host.PersistentStorage{
			Device:          "/dev/sda4",
			DiskBytes:       diskBytes,
			PartitionBytes:  diskBytes - 1100<<20,
			FilesystemBytes: imageBytes,
		}, nil)
		diags := checkStorageGrown()
		if len(diags) != 1 {
			t.Fatalf("diagnostics = %+v, want one", diags)
		}
		if !strings.Contains(diags[0].Message, "uses 944 MiB of a 238 GiB disk") {
			t.Errorf("message %q does not give the size the filesystem holds", diags[0].Message)
		}
	})

	t.Run("grown", func(t *testing.T) {
		persistentStorageAnswers(t, host.PersistentStorage{
			Device:          "/dev/sda4",
			DiskBytes:       diskBytes,
			PartitionBytes:  diskBytes - 1100<<20,
			FilesystemBytes: diskBytes - 1100<<20,
			Grown:           true,
		}, nil)
		if diags := checkStorageGrown(); len(diags) != 0 {
			t.Fatalf("grown storage reported %+v", diags)
		}
	})

	t.Run("no persistent storage", func(t *testing.T) {
		persistentStorageAnswers(t, host.PersistentStorage{}, host.ErrNoPersistentStorage)
		if diags := checkStorageGrown(); len(diags) != 0 {
			t.Fatalf("a system with no persistent storage reported %+v", diags)
		}
	})

	t.Run("state cannot be read", func(t *testing.T) {
		persistentStorageAnswers(t, host.PersistentStorage{}, errors.New("read /sys/class/block/sda4/size: permission denied"))
		diags := checkStorageGrown()
		if len(diags) != 1 {
			t.Fatalf("diagnostics = %+v, want one: an unread state is not a healthy one", diags)
		}
		got := diags[0]
		if got.Code != diagnostic.CodeDoctorStorageNotGrown || got.Severity != diagnostic.SeverityWarning {
			t.Errorf("finding is %s %s, want a %s warning", got.Severity, got.Code, diagnostic.CodeDoctorStorageNotGrown)
		}
		const message = "cannot read the size of the persistent storage, so it may be smaller than its disk: " +
			"read /sys/class/block/sda4/size: permission denied"
		if got.Message != message {
			t.Errorf("message = %q, want %q", got.Message, message)
		}
		if got.Fix != "show host storage" {
			t.Errorf("fix = %q, want the command that reads the sizes", got.Fix)
		}
	})
}

// TestBinarySize pins the unit boundaries of the sizes a storage finding
// prints, and the one decimal it keeps below ten units.
func TestBinarySize(t *testing.T) {
	for _, tc := range []struct {
		byteCount uint64
		want      string
	}{
		{0, "0 bytes"},
		{1023, "1023 bytes"},
		{1024, "1 KiB"},
		{1<<20 - 1, "1023 KiB"},
		{1 << 20, "1 MiB"},
		{944 << 20, "944 MiB"},
		{1<<30 - 1, "1023 MiB"},
		{1 << 30, "1 GiB"},
		{2040109466, "1.9 GiB"},
		{2040109465, "1.8 GiB"},
		{10 << 30, "10 GiB"},
		{10<<30 + 900<<20, "10 GiB"},
		{238 << 30, "238 GiB"},
		{1 << 40, "1 TiB"},
		{1<<64 - 1, "16777215 TiB"},
	} {
		if got := binarySize(tc.byteCount); got != tc.want {
			t.Errorf("binarySize(%d) = %q, want %q", tc.byteCount, got, tc.want)
		}
	}
}

// TestStorageGrownCheckRunsOnTheApplianceOnly asks the registry which
// platforms the storage-grown check runs on.
//
// VALIDATES: the check is registered for the appliance platform and for no
// other, so a host that is not an appliance is never asked about a persistent
// partition it does not have.
// PREVENTS: the check running everywhere, or nowhere, after an edit to its
// platform list.
func TestStorageGrownCheckRunsOnTheApplianceOnly(t *testing.T) {
	checks := diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePreConfig)
	at := slices.IndexFunc(checks, func(got diagnostic.DoctorCheck) bool { return got.Name == "storage-grown" })
	if at < 0 {
		t.Fatal("the registry holds no storage-grown check in the pre-config phase")
	}
	check := checks[at]
	if !diagnostic.DoctorCheckSupportsPlatform(check, &host.PlatformInfo{Type: host.PlatformGokrazy}) {
		t.Error("the check does not run on the appliance platform")
	}
	for _, platform := range []host.PlatformType{
		host.PlatformUnknown, host.PlatformSystemd, host.PlatformContainer, host.PlatformPlainLinux, host.PlatformDarwin,
	} {
		if diagnostic.DoctorCheckSupportsPlatform(check, &host.PlatformInfo{Type: platform}) {
			t.Errorf("the check runs on platform %s, which has no persistent partition", platform)
		}
	}
	if diagnostic.DoctorCheckSupportsPlatform(check, nil) {
		t.Error("the check runs when the platform was not detected")
	}
}

// TestInstallerSpellsTheStorageDoctorCode compares the two constants the
// installer spells with the ones this check uses. The installer links no
// component, so it cannot import them, and its log line is the first place an
// operator meets the code. The test reads the installer's source, because the
// constants are unexported there.
//
// VALIDATES: the installer logs a failed growth under the code this check
// declares, with the fix this check names.
// PREVENTS: a rename on one side leaving the installer's log line naming a
// code `ze show diagnostic` cannot resolve.
func TestInstallerSpellsTheStorageDoctorCode(t *testing.T) {
	const installerSource = "../../install/disk/run.go"
	file, err := parser.ParseFile(token.NewFileSet(), installerSource, nil, 0)
	if err != nil {
		t.Fatalf("parse the installer source: %v", err)
	}
	spelled := map[string]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		if gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range value.Names {
				if i >= len(value.Values) {
					continue
				}
				literal, ok := value.Values[i].(*ast.BasicLit)
				if !ok {
					continue
				}
				text, unquoteErr := strconv.Unquote(literal.Value)
				if unquoteErr != nil {
					continue // A constant that is not a string is not one of the two.
				}
				spelled[name.Name] = text
			}
		}
	}
	for name, want := range map[string]string{
		"codeStorageNotGrown": diagnostic.CodeDoctorStorageNotGrown,
		"fixStorageGrow":      fixStorageGrow,
	} {
		got, ok := spelled[name]
		if !ok {
			t.Errorf("%s declares no string constant %s", installerSource, name)
			continue
		}
		if got != want {
			t.Errorf("%s spells %s as %q, the doctor check uses %q", installerSource, name, got, want)
		}
	}
}

// TestDoctorStorageGrownCheckFunctional drives the storage-grown check through
// the real entry point, on a platform detection forced to answer the
// appliance and a persistent storage read forced to answer storage smaller
// than its disk.
//
// VALIDATES: `ze doctor --json` prints doctor-storage-not-grown on an
// appliance, so the registered check is what the runner reaches, and prints
// none on a plain Linux host given the same storage answer.
// PREVENTS: a check declared in the table, unit tested, and never run.
func TestDoctorStorageGrownCheckFunctional(t *testing.T) {
	persistentStorageAnswers(t, host.PersistentStorage{
		Device:          "/dev/sda4",
		DiskBytes:       238 << 30,
		PartitionBytes:  944 << 20,
		FilesystemBytes: 944 << 20,
	}, nil)
	t.Cleanup(func() { _ = env.Set(doctorPlatformEnv, "") })

	if err := env.Set(doctorPlatformEnv, "gokrazy"); err != nil {
		t.Fatal(err)
	}
	codes := doctorCodesFor(t, minimalConfig)
	if !slices.Contains(codes, diagnostic.CodeDoctorStorageNotGrown) {
		t.Fatalf("codes on the appliance = %v, want %q among them", codes, diagnostic.CodeDoctorStorageNotGrown)
	}

	if err := env.Set(doctorPlatformEnv, "plain-linux"); err != nil {
		t.Fatal(err)
	}
	codes = doctorCodesFor(t, minimalConfig)
	if slices.Contains(codes, diagnostic.CodeDoctorStorageNotGrown) {
		t.Fatalf("codes on a plain Linux host = %v, want no %q", codes, diagnostic.CodeDoctorStorageNotGrown)
	}
}
