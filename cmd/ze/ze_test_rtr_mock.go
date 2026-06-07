// Design: docs/architecture/testing/ci-format.md -- RTR mock wiring

//go:build ze_test

package main

import "codeberg.org/thomas-mangin/ze/internal/test/mock/rtr"

func zeTestRtrMockCmd(args []string) int { return rtr.Run(args) }
