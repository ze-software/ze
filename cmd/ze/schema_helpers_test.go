//go:build ze_core

package main

import (
	"testing"

	cli "github.com/ze-software/ze/internal/component/cli/client"
)

// The helpers below call the functions whose error is the YANG loader's
// refusal of the schema, and fail the test on it. Every test in this package
// runs over the shipped schema, so an error there is a broken tree, never an
// answer a test may read as empty.

func yangTreeForTest(t testing.TB) *cli.Command {
	t.Helper()
	tree, err := cli.YANGCommandTree()
	if err != nil {
		t.Fatalf("YANGCommandTree: %v", err)
	}
	return tree
}

func commandTreeForTest(t testing.TB) *cli.Command {
	t.Helper()
	tree, err := cli.BuildCommandTree(false)
	if err != nil {
		t.Fatalf("BuildCommandTree: %v", err)
	}
	return tree
}
