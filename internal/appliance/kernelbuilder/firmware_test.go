package kernelbuilder

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// firmwareFixture writes body as the blob's file under a fresh firmware
// directory, and a kernel .config holding one line under a fresh build tree.
func firmwareFixture(t *testing.T, blob FirmwareBlob, body []byte) (firmwareDir, buildTree string) {
	t.Helper()
	firmwareDir = t.TempDir()
	path := filepath.Join(firmwareDir, filepath.FromSlash(blob.Path))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	buildTree = t.TempDir()
	if err := os.WriteFile(filepath.Join(buildTree, ".config"), []byte("CONFIG_X=y\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return firmwareDir, buildTree
}

func firmwareTestDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// VALIDATES: the worker compiles a firmware blob into the kernel only when its
// SHA-256 matches its pin, and refuses one that does not before the kernel
// config names it, saying which file, both digests, and the fetch of the
// pinned blob.
// PREVENTS: a hand-placed or altered blob in --firmware-dir reaching
// CONFIG_EXTRA_FIRMWARE, where the build would compile bytes nobody reviewed.
// Method: a synthetic pin, its file written once with other bytes and once with
// the pinned ones, through embedFirmwareBlobs, the step embedFirmware runs.
func TestEmbedFirmwareRefusesUnpinnedBlob(t *testing.T) {
	pinned := []byte("the pinned firmware")
	blob := FirmwareBlob{Path: "i915/blob.bin", Commit: strings.Repeat("a", 40), SHA256: firmwareTestDigest(pinned)}
	served := []byte("other firmware")
	firmwareDir, buildTree := firmwareFixture(t, blob, served)
	req := WorkerRequest{Profile: hardwareKMSProfile, FirmwareDir: firmwareDir, Stdout: io.Discard}

	err := embedFirmwareBlobs(req, buildTree, []FirmwareBlob{blob})
	if err == nil {
		t.Fatal("a firmware blob whose SHA-256 differs from its pin was compiled in")
	}
	path := filepath.Join(firmwareDir, "i915", "blob.bin")
	for _, want := range []string{path, firmwareTestDigest(served), blob.SHA256, blob.Commit, "curl -fL -o " + shellQuote(path) + " " + shellQuote(blob.URL())} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q:\n%v", want, err)
		}
	}
	config, readErr := os.ReadFile(filepath.Join(buildTree, ".config"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(config) != "CONFIG_X=y\n" {
		t.Errorf("a refused blob still reached the kernel config:\n%s", config)
	}
}

// VALIDATES: a blob whose SHA-256 matches its pin is named in
// CONFIG_EXTRA_FIRMWARE, with the firmware directory it is read from.
// PREVENTS: a digest check that refuses every blob, which the refusal test
// alone would not notice.
func TestEmbedFirmwareCompilesInPinnedBlob(t *testing.T) {
	pinned := []byte("the pinned firmware")
	blob := FirmwareBlob{Path: "i915/blob.bin", Commit: strings.Repeat("b", 40), SHA256: firmwareTestDigest(pinned)}
	firmwareDir, buildTree := firmwareFixture(t, blob, pinned)
	req := WorkerRequest{Profile: hardwareKMSProfile, FirmwareDir: firmwareDir, Stdout: io.Discard}

	if err := embedFirmwareBlobs(req, buildTree, []FirmwareBlob{blob}); err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile(filepath.Join(buildTree, ".config"))
	if err != nil {
		t.Fatal(err)
	}
	want := "CONFIG_X=y\nCONFIG_EXTRA_FIRMWARE=\"i915/blob.bin\"\nCONFIG_EXTRA_FIRMWARE_DIR=\"" + firmwareDir + "\"\n"
	if string(config) != want {
		t.Errorf("kernel config = %q, want %q", config, want)
	}
}

// VALIDATES: the provenance of a hardware-kms kernel names each firmware blob
// it compiled in, by the URL of its pinned commit and its SHA-256.
// PREVENTS: a record that names the kernel source and not the firmware built
// into the same image.
// The expected lines are literal, so this golden does not read the pin: a
// re-pin updates it.
func TestWriteProvenanceRecordsFirmware(t *testing.T) {
	root := t.TempDir()
	req := Request{Root: root, Version: "7.3", Arch: "amd64", Profile: hardwareKMSProfile, Target: "installer", Modules: "no"}
	if err := writeProvenance("out", req, "docker"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "out", ProvenanceName))
	if err != nil {
		t.Fatal(err)
	}
	want := "firmware-url=https://git.kernel.org/pub/scm/linux/kernel/git/firmware/linux-firmware.git/plain/i915/adlp_dmc.bin?id=41e615cf3479ea5d9daee1a7b7f0920c84acec10\n" +
		"firmware-sha256=3516de2e134ddcf3b319c75d2e437779fecbd58cbb77234bd6f297c544e92ccb\n"
	if !strings.HasSuffix(string(data), want) {
		t.Errorf("provenance %q does not end with the firmware pins %q", data, want)
	}

	req.Profile = "hardware"
	if err := writeProvenance("out", req, "docker"); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(root, "out", ProvenanceName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "firmware") {
		t.Errorf("the provenance of a profile that compiles in no firmware names some:\n%s", data)
	}
}
