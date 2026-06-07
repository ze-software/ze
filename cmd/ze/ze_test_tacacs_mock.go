// Design: docs/architecture/testing/ci-format.md -- TACACS+ mock wiring

//go:build ze_test

package main

import "codeberg.org/thomas-mangin/ze/internal/test/mock/tacacs"

func zeTestTacacsMockCmd(args []string) int { return tacacs.Run(args) }
