//go:build ze_core

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/cmd/ze/startprep"
)

// TestStartRunsTheStartStepsBeforeTheStore proves `ze start` runs every
// registered start step before it reaches its first store or interface work,
// so a step that makes a NIC appear runs before anything lists the NICs.
//
// Method: register a recording step, then start with --web-only and a config
// path, which cmdStart refuses at its first config-path branch, before it
// opens any store.
// MUTATION: moving startprep.Run below the explicit-file branch, or deleting
// it, returns 1 with the step never run.
func TestStartRunsTheStartStepsBeforeTheStore(t *testing.T) {
	ran := 0
	t.Cleanup(startprep.Register("test-start-step", func() { ran++ }))

	code := cmdStart([]string{flagStartWebOnly, "router.conf"}, nil, 0, -1, "", "", "", false, false)

	assert.Equal(t, 1, code, "--web-only with a config path is refused")
	assert.Equal(t, 1, ran, "the start step runs once, before the refusal")
}
