// Design: docs/architecture/api/process-protocol.md -- OPEN Validation
// Related: validate.go -- broadcastStampedValidateOpen, which this calls

package server

import pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

// broadcastValidateOpen asks the plugins about an OPEN stamped with no
// generation: the tests that use it judge the answers, never how an OPEN is
// ordered against state events.
func broadcastValidateOpen(s *pluginserver.Server, peerAddr, group string, policyPlugins []string, local, remote any) error {
	return broadcastStampedValidateOpen(s, peerAddr, group, 0, policyPlugins, local, remote)
}
