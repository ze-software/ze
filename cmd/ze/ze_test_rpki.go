// Design: docs/architecture/testing/ci-format.md -- RPKI mock wiring

//go:build ze_test

package main

import "codeberg.org/thomas-mangin/ze/internal/test/mock/rpki"

func zeTestRpkiCmd(args []string) int { return rpki.Run(args) }
