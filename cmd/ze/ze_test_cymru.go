// Design: docs/architecture/testing/ci-format.md -- Cymru DNS mock wiring

//go:build ze_test

package main

import "codeberg.org/thomas-mangin/ze/internal/test/mock/cymru"

func zeTestCymruCmd(args []string) int { return cymru.Run(args) }
