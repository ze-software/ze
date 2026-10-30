//go:build ze_pxe

// Design: docs/architecture/testing/ci-format.md -- tree storage functional scenarios
// Related: storage_init_from_fixture.go -- storageInitFromRemote, the import check this scenario shares
// Related: register_storage_init_from_tftp.go -- registers the scenario

package fixture

import (
	"context"
	"crypto/sha256"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/plugins/tftpserver"
)

// storageInitFromTFTP serves a seed blob from Ze's own TFTP server on a
// loopback port and runs `ze init --from tftp://127.0.0.1:<port>/...
// --sha256 <hex>`: Ze's TFTP client fetches it, the digest and the blob are
// checked, the tree is imported, the fetched copy is removed, and `ze start`
// runs on the tree.
func storageInitFromTFTP(ctx context.Context, _ []string) error {
	values := storageInitFromValues()
	blob, err := storageInitFromSeed(values)
	if err != nil {
		return err
	}
	root, err := os.MkdirTemp("", "ze-init-from-tftp-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root) //nolint:errcheck // a leftover temporary folder changes no result
	if err := os.MkdirAll(filepath.Join(root, "install"), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "install", "database.zefs"), blob, 0o600); err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		return err
	}
	stop := tftpserver.ServeConn(conn, root, 2, slogutil.DiscardLogger())
	defer stop()

	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return errors.New("the loopback TFTP socket has no UDP address")
	}
	port := addr.Port
	sum := sha256.Sum256(blob)
	source := "tftp://127.0.0.1:" + strconv.Itoa(port) + "/install/database.zefs"
	if err := storageInitFromRemote(ctx, source, textbuf.StringHex(sum[:]), values); err != nil {
		return err
	}
	return storageInitFromOK("init-from-tftp")
}
