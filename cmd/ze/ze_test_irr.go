// Design: docs/architecture/testing/ci-format.md -- IRR whois mock wiring

//go:build ze_test

package main

import "codeberg.org/thomas-mangin/ze/internal/test/mock/irr"

func zeTestIrrCmd(args []string) int { return irr.Run(args) }
