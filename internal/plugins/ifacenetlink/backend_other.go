// Design: docs/features/interfaces.md -- Non-Linux interface backend stub
// Overview: ifacenetlink.go -- package hub

//go:build !linux

package ifacenetlink

import (
	"fmt"
	"runtime"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// stubBackend implements iface.Backend on non-Linux platforms.
// All operations return "not supported" errors.
type stubBackend struct{}

func newNetlinkBackend() (iface.Backend, error) {
	return &stubBackend{}, nil
}

func unsupported() error {
	return fmt.Errorf("interface management not supported on %s", runtime.GOOS)
}

func (s *stubBackend) CreateDummy(_ string) error                             { return unsupported() }
func (s *stubBackend) CreateVeth(_, _ string) error                           { return unsupported() }
func (s *stubBackend) CreateBridge(_ string) error                            { return unsupported() }
func (s *stubBackend) CreateVLAN(_ string, _ int) error                       { return unsupported() }
func (s *stubBackend) CreateTunnel(_ iface.TunnelSpec) error                  { return unsupported() }
func (s *stubBackend) CreateWireguardDevice(_ string) error                   { return unsupported() }
func (s *stubBackend) DeleteInterface(_ string) error                         { return unsupported() }
func (s *stubBackend) AddAddress(_, _ string) error                           { return unsupported() }
func (s *stubBackend) RemoveAddress(_, _ string) error                        { return unsupported() }
func (s *stubBackend) ReplaceAddressWithLifetime(_, _ string, _, _ int) error { return unsupported() }
func (s *stubBackend) SetAdminUp(_ string) error                              { return unsupported() }
func (s *stubBackend) SetAdminDown(_ string) error                            { return unsupported() }
func (s *stubBackend) SetMTU(_ string, _ int) error                           { return unsupported() }
func (s *stubBackend) SetMACAddress(_, _ string) error                        { return unsupported() }
func (s *stubBackend) GetMACAddress(_ string) (string, error)                 { return "", unsupported() }
func (s *stubBackend) GetStats(_ string) (*iface.InterfaceStats, error)       { return nil, unsupported() }
func (s *stubBackend) ListInterfaces() ([]iface.InterfaceInfo, error)         { return nil, unsupported() }
func (s *stubBackend) GetInterface(_ string) (*iface.InterfaceInfo, error)    { return nil, unsupported() }
func (s *stubBackend) BridgeAddPort(_, _ string) error                        { return unsupported() }
func (s *stubBackend) BridgeDelPort(_ string) error                           { return unsupported() }
func (s *stubBackend) BridgeSetSTP(_ string, _ bool) error                    { return unsupported() }
func (s *stubBackend) SetIPv4Forwarding(_ string, _ bool) error               { return unsupported() }
func (s *stubBackend) SetIPv4ArpFilter(_ string, _ bool) error                { return unsupported() }
func (s *stubBackend) SetIPv4ArpAccept(_ string, _ bool) error                { return unsupported() }
func (s *stubBackend) SetIPv4ProxyARP(_ string, _ bool) error                 { return unsupported() }
func (s *stubBackend) SetIPv4ArpAnnounce(_ string, _ int) error               { return unsupported() }
func (s *stubBackend) SetIPv4ArpIgnore(_ string, _ int) error                 { return unsupported() }
func (s *stubBackend) SetIPv4RPFilter(_ string, _ int) error                  { return unsupported() }
func (s *stubBackend) SetIPv6Autoconf(_ string, _ bool) error                 { return unsupported() }
func (s *stubBackend) SetIPv6AcceptRA(_ string, _ int) error                  { return unsupported() }
func (s *stubBackend) SetIPv6Forwarding(_ string, _ bool) error               { return unsupported() }
func (s *stubBackend) SetupMirror(_, _ string, _, _ bool) error               { return unsupported() }
func (s *stubBackend) RemoveMirror(_ string) error                            { return unsupported() }
func (s *stubBackend) StartMonitor(_ ze.EventBus) error                       { return unsupported() }
func (s *stubBackend) StopMonitor()                                           {}
func (s *stubBackend) Close() error                                           { return nil }
