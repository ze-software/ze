// Design: docs/contributing/gh-pages.md -- renderer peers share the interop release pins.
// Related: render.go -- BuildImage passes these pins to the renderer Dockerfile.
package siteterminaldemo

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const rendererKeepalivedPin = "test/interop/Dockerfile.keepalived"

var rendererRelease = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// rendererPeerArguments reads the declarations the labs actually build, rather
// than maintaining another release list in the renderer. The source-pin parser
// handles ZE_SOURCE_* only; these are stable-release ARG defaults, not commits.
func rendererPeerArguments(root string) ([]string, error) {
	_, tagged, found := strings.Cut(interoplab.FRRImage, ":")
	if !found {
		return nil, fmt.Errorf("interoplab.FRRImage has no release tag: %q", interoplab.FRRImage)
	}
	frr, _, _ := strings.Cut(tagged, "@")
	if !rendererRelease.MatchString(frr) {
		return nil, fmt.Errorf("interoplab.FRRImage has no stable release tag: %q", interoplab.FRRImage)
	}
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rendererKeepalivedPin))) //nolint:gosec // a build tool reads the checkout it was pointed at
	if err != nil {
		return nil, fmt.Errorf("read renderer peer pin: %w", err)
	}
	declarations := string(body)
	version, err := rendererPinArgument(declarations, "KEEPALIVED_VERSION")
	if err != nil {
		return nil, err
	}
	if !rendererRelease.MatchString(version) {
		return nil, fmt.Errorf("%s: KEEPALIVED_VERSION is not a stable release: %q", rendererKeepalivedPin, version)
	}
	checksum, err := rendererPinArgument(declarations, "KEEPALIVED_SHA256")
	if err != nil {
		return nil, err
	}
	decoded, err := hex.DecodeString(checksum)
	if err != nil {
		return nil, fmt.Errorf("%s: KEEPALIVED_SHA256 is not hexadecimal: %w", rendererKeepalivedPin, err)
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("%s: KEEPALIVED_SHA256 must contain 32 bytes", rendererKeepalivedPin)
	}
	return []string{"FRR_VERSION=" + frr, "KEEPALIVED_VERSION=" + version, "KEEPALIVED_SHA256=" + checksum}, nil
}

// rendererPinLabels maps each build argument rendererPeerArguments answers to
// the label demos/terminal/Dockerfile records its value under. The Docker
// proof (image_build_docker_test.go) builds through that Dockerfile and reads
// the labels back, so the two declarations cannot drift apart unseen.
var rendererPinLabels = map[string]string{
	"FRR_VERSION":        "ze.renderer.frr",
	"KEEPALIVED_VERSION": "ze.renderer.keepalived",
	"KEEPALIVED_SHA256":  "ze.renderer.keepalived.sha256",
}

// requireRendererPins refuses a renderer image whose labels do not carry the
// peer pins the checkout declares now: an image built before a pin bump would
// record demos against a peer release the labs no longer run.
func (e *Engine) requireRendererPins(image string, labels map[string]string) error {
	arguments, err := rendererPeerArguments(e.root)
	if err != nil {
		return err
	}
	for _, argument := range arguments {
		name, want, _ := strings.Cut(argument, "=")
		label, known := rendererPinLabels[name]
		if !known {
			return fmt.Errorf("BUG: renderer peer argument %s has no image label", name)
		}
		got, labeled := labels[label]
		if labeled && got == want {
			continue
		}
		var buffer textbuf.Buffer
		buffer.Str("the renderer image ").Str(image).Str(" was built from other peer pins: its label ").Str(label)
		if labeled {
			buffer.Str(" is \"").Str(got).Str("\"")
		} else {
			buffer.Str(" is missing")
		}
		buffer.Str(", and the checkout pins ").Str(argument).
			Str("; rebuild it with: le site terminal-demo build image")
		return errors.New(buffer.String())
	}
	return nil
}

// rendererPinArgument accepts the literal ARG NAME=value declarations used by
// the canonical peer Dockerfile. Missing, empty or duplicate pins refuse a build.
func rendererPinArgument(body, name string) (string, error) {
	var value string
	for line := range strings.SplitSeq(body, "\n") {
		words := strings.Fields(line)
		if len(words) < 2 {
			continue
		}
		if !strings.EqualFold(words[0], "ARG") {
			continue
		}
		key, declared, found := strings.Cut(words[1], "=")
		if key != name {
			continue
		}
		if len(words) != 2 {
			return "", fmt.Errorf("%s: %s must be one literal ARG NAME=value declaration", rendererKeepalivedPin, name)
		}
		if !found {
			return "", fmt.Errorf("%s: %s has no default", rendererKeepalivedPin, name)
		}
		if declared == "" {
			return "", fmt.Errorf("%s: %s is empty", rendererKeepalivedPin, name)
		}
		if value != "" {
			return "", fmt.Errorf("%s: %s is declared more than once", rendererKeepalivedPin, name)
		}
		value = declared
	}
	if value == "" {
		return "", fmt.Errorf("%s: missing ARG %s=value; restore the canonical peer pin before running ./le site terminal-demo build image", rendererKeepalivedPin, name)
	}
	return value, nil
}
