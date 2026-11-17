// Design: docs/architecture/container/image-store.md -- the writable layer folder the image store hands a container
// Related: main.go -- the verify and gone modes that read the folder found here

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// storeLayersFolder is where the image store keeps the writable layers,
// under its data directory: one folder per container, and in it one folder
// per image the container ran, named by that image's digest.
const storeLayersFolder = "container/layers"

// layerDigestPrefix starts the name of a writable layer folder: the digest's
// algorithm and a hyphen, then the digest's hex.
const layerDigestPrefix = "sha256-"

// layerDigestHexLength is the length of a sha256 digest in hex.
const layerDigestHexLength = 64

// storedUpper waits, a bounded time, until the image store under data holds
// a writable layer folder for container name, then requires exactly one,
// named by an image digest, and answers its upper folder, where the overlay
// puts what the container wrote. On any other finding it prints a FAIL line
// naming what it found and answers false: no folder after the wait, more
// than one, or one whose name is no digest.
func storedUpper(data, name string) (string, bool) {
	folder := filepath.Join(data, storeLayersFolder, name)
	for range reportWaitSteps {
		entries, err := os.ReadDir(folder)
		if errors.Is(err, os.ErrNotExist) {
			time.Sleep(reportWaitStep)
			continue
		}
		if err != nil {
			return "", judge("the writable layers of "+name+" can be read", false, err.Error())
		}
		if len(entries) == 0 {
			time.Sleep(reportWaitStep)
			continue
		}
		return judgeLayers(folder, entries)
	}
	waited := (reportWaitStep * reportWaitSteps).String()
	return "", judge("a writable layer under "+folder+" within "+waited+", which the store makes when it answers the image", false, "none")
}

// judgeLayers requires entries, the content of folder, to be one folder
// named by an image digest, and answers the upper folder inside it.
func judgeLayers(folder string, entries []os.DirEntry) (string, bool) {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if !judge("one writable layer under "+folder, len(entries) == 1, strings.Join(names, ",")) {
		return "", false
	}
	hex, prefixed := strings.CutPrefix(names[0], layerDigestPrefix)
	if !judge("the writable layer is named by a sha256 digest", prefixed, names[0]) {
		return "", false
	}
	if !judge("the writable layer's digest is 64 lowercase hex digits", isDigestHex(hex), names[0]) {
		return "", false
	}
	if !judge("the writable layer is a folder", entries[0].IsDir(), entries[0].Type().String()) {
		return "", false
	}
	return filepath.Join(folder, names[0], "upper"), true
}

// isDigestHex answers whether text is a sha256 digest's hex: 64 digits and
// lowercase letters from a to f.
func isDigestHex(text string) bool {
	if len(text) != layerDigestHexLength {
		return false
	}
	// Bounded by layerDigestHexLength, checked above.
	for i := range len(text) {
		if !isLowerHexByte(text[i]) {
			return false
		}
	}
	return true
}

func isLowerHexByte(c byte) bool {
	if c >= '0' && c <= '9' {
		return true
	}
	return c >= 'a' && c <= 'f'
}
