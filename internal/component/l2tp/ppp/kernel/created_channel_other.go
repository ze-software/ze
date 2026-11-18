// Design: docs/research/l2tpv2-ze-integration.md -- original PPPoX channel capture

//go:build !linux

package kernel

import "os"

func CreatePPPChannel(*os.File) (*CreatedChannel, error) { return nil, errNotLinux }
