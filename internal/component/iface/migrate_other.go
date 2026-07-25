// Design: docs/features/interfaces.md — Make-before-break interface migration
// Overview: migrate.go — MigrateConfig type

//go:build !linux

package iface

import (
	"errors"
	"time"

	"github.com/ze-software/ze/pkg/ze"
)

var errInterfaceMigrationIsOnlySupportedOn = errors.New("interface migration is only supported on Linux")

// MigrateInterface is not supported on non-Linux platforms.
func MigrateInterface(_ MigrateConfig, _ ze.EventBus, _ time.Duration) error {
	return errInterfaceMigrationIsOnlySupportedOn
}
