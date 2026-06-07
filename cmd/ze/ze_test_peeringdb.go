// Design: docs/architecture/testing/ci-format.md -- PeeringDB mock wiring

//go:build ze_test

package main

import "codeberg.org/thomas-mangin/ze/internal/test/mock/peeringdb"

func zeTestPeeringdbCmd(args []string) int { return peeringdb.Run(args) }
