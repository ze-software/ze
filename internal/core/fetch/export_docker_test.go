//go:build docker && linux

// Design: docs/architecture/testing/interop.md -- a package test's own server container (labtest)
// Related: tftp_dnsmasq_docker_test.go -- the interop test this file serves
// Related: tftp.go -- the transfer it runs

package fetch

import (
	"io"
	"net"

	"github.com/ze-software/ze/internal/core/tftp"
)

// RunTFTPTransferForTest runs one TFTP read request for file against server over
// conn, writes the file into out, and answers the options the server granted.
//
// It exists for TestTFTPClientDnsmasqInterop alone. That test lives in package
// fetch_test because its lab harness (internal/le/interoplab) reaches
// internal/component/config/system, which imports this package, so an
// in-package test importing the harness is an import cycle. Only a test binary
// built with the docker tag compiles this file.
func RunTFTPTransferForTest(conn *net.UDPConn, server *net.UDPAddr, out io.Writer, file string) (tftp.Negotiated, error) {
	transfer := newTFTPTransfer(conn, server, out)
	err := transfer.run(file)
	return transfer.granted, err
}
