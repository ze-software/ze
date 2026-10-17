//go:build zetest && ze_core

// Design: docs/functional-tests.md -- the DUT's store
// Related: ze_core_start.go -- openExplicitStore, createAbsentStore.

package main

import (
	"errors"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// testStorageKey selects the store a functional-test DUT creates when none
// exists. Only this file, built under the zetest tag that no shipped build
// carries, reads it.
const testStorageKey = "ze.test.storage"

// testStorageMemory is the one value testStorageKey accepts besides empty.
const testStorageMemory = "memory"

func init() {
	_ = env.MustRegister(env.EnvEntry{
		Key:         testStorageKey,
		Type:        "string",
		Description: "Test DUT builds only: `memory` runs an absent store in process memory (test infrastructure)",
		Private:     true,
	})
	createAbsentStore = createTestStore
}

// createTestStore is createAbsentStore in a DUT build. With testStorageKey set
// to memory it returns storage.CreateMemory, whose keys die with the process
// and whose writes never sync, so a test's start does not wait on the disk.
// An existing store is never reached here: openExplicitStore opens it first,
// so a test that seeds a store keeps it.
func createTestStore(dir string) (storage.Storage, error) {
	switch value := env.Get(testStorageKey); value {
	case "":
		return createLiveStore(dir)
	case testStorageMemory:
		var tb textbuf.Buffer
		tb.Str("created memory store (test build): ").Str(dir).Str("\n").StdErr() //nolint:errcheck // a lost notice line does not change the store
		return storage.CreateMemory(), nil
	default:
		return nil, errTestStorage(value)
	}
}

// errTestStorage names the value and the two that work.
func errTestStorage(value string) error {
	var tb textbuf.Buffer
	return errors.New(tb.Str(testStorageKey).Str(" = ").Str(value).Str(": unknown store; set it to `memory`, or leave it unset for the disk store").String())
}
