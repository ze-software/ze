// Design: docs/features/interfaces.md — Make-before-break interface migration
// Overview: migrate.go — MigrateConfig type

//go:build !linux

package iface

import (
	"fmt"
	"time"

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// MigrateInterface is not supported on non-Linux platforms.
func MigrateInterface(_ MigrateConfig, _ ze.Bus, _ time.Duration) error {
	return fmt.Errorf("interface migration is only supported on Linux")
}
