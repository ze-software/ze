// Design: docs/architecture/provisioning/image-server.md -- image server plugin registration

package imageserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/version"
	imgyang "github.com/ze-software/ze/internal/plugins/imageserver/yang"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const configRootService = "service"

var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	loggerPtr.Store(slogutil.DiscardLogger())

	// StartAfter: the server binds each listen-interface in its Stage 2
	// configure, and the interface plugin creates the VLANs, bridges and dummies
	// of ze's own `interface {}` config in its own Stage 2 configure, so the
	// server starts in a later tier whenever both run. It is an order, not a
	// dependency: a config with no `interface` section does not load interface.
	reg := registry.Registration{
		Name:                    "imageserver",
		Description:             "Image server: HTTP provisioning for disk images and boot files",
		Features:                "yang",
		YANG:                    imgyang.ZeImageServerConfYANG,
		ConfigRoots:             []string{configRootService},
		StartAfter:              []string{"interface"},
		InProcessConfigVerifier: verifyImageConfig,
		RunEngine:               runImageServerPlugin,
	}
	reg.CLIHandler = func(_ []string) int { return 1 }
	reg.ConfigureEngineLogger = func(loggerName string) {
		l := slogutil.Logger(loggerName)
		if l != nil {
			loggerPtr.Store(l)
		}
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "imageserver: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func verifyImageConfig(sections []sdk.ConfigSection) error {
	_, _, err := parseImageSections(sections)
	return err
}

func runImageServerPlugin(conn net.Conn) int {
	log := loggerPtr.Load()
	log.Debug("imageserver plugin starting")

	p := sdk.NewWithConn("imageserver", conn)
	defer closeLogged(p, log, "plugin conn")

	srv := &imageServer{log: log}
	tx := sdk.NewConfigTx(imageConfig{}, parseImageSections, srv.install)
	tx.Register(p)
	p.OnConfigure(func(sections []sdk.ConfigSection) error { return srv.configureAtBoot(tx, sections) })

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, &sdk.Registration{
		WantsConfig:  []string{configRootService},
		VerifyBudget: 2,
		ApplyBudget:  5,
	}); err != nil {
		log.Error("imageserver plugin failed", "error", err)
		srv.stop()
		return 1
	}

	srv.stop()
	log.Info("imageserver plugin stopped")
	return 0
}

// listenTarget is one address the image server binds, with the listen-interface
// entry the operator wrote for it so a failure names the interface rather than
// the address alone. An empty iface is the "no interface named" case, whose
// empty ip binds every address of the host.
type listenTarget struct {
	iface string
	ip    string
}

// imageResolveIPv4 resolves a listen-interface and imageServeTarget binds
// one target (vars so tests can stub the resolve and the bind).
var (
	imageResolveIPv4 = resolveInterfaceIPv4
	imageServeTarget = serveTarget
)

// bindImage resolves and binds every listen-interface of cfg. A disabled cfg
// binds nothing.
//
// strict is the reload mode: it binds every interface or none, and the error
// names the interface and the fix, so the transaction rolls back to the
// config that served. Boot is not strict: it has no previous config to keep,
// a configure error at boot fails the plugin and aborts the whole daemon
// start, and an interface's IPv4 address may arrive later through DHCP, so a
// failed interface is logged and the others serve. A device ze's own
// `interface {}` config creates exists by then, because the server starts
// after the interface plugin (StartAfter in init); an address or a device
// that appears later is served only after the next reload.
func bindImage(cfg imageConfig, zefsPath string, log *slog.Logger, strict bool) ([]*http.Server, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	targets, err := listenTargets(cfg.ListenInterfaces, log, strict)
	if err != nil {
		return nil, err
	}
	servers, err := startTargets(cfg, zefsPath, targets, log, strict)
	if err != nil {
		return nil, err
	}
	if len(servers) == 0 {
		// Every named interface failed to resolve or to bind. Report that
		// instead of logging "started" over a server that serves nothing.
		log.Error("imageserver: no interfaces bound; server not serving",
			"interfaces", cfg.ListenInterfaces)
	}
	return servers, nil
}

// listenTargets resolves every entry of listen-interface to its IPv4 address,
// one target for each entry. The leaf is a leaf-list, so every entry is served:
// internal/plugins/tftpserver/config_tx.go and
// internal/plugins/dhcpserver/register.go read the same leaf the same way, and
// an operator who runs the three services for one PXE install gets all of them
// on the interfaces named. Not strict, an entry that does not resolve is
// reported and skipped, so one wrong name does not stop the interfaces that do
// resolve; strict, it is the error. With no entry at all the result is one
// target with an empty address, which binds every address of the host.
func listenTargets(names []string, log *slog.Logger, strict bool) ([]listenTarget, error) {
	if len(names) == 0 {
		return []listenTarget{{}}, nil
	}

	targets := make([]listenTarget, 0, len(names))
	for _, name := range names {
		ip, err := imageResolveIPv4(name)
		if err != nil && strict {
			return nil, unboundImageInterface(name, err)
		}
		if err != nil {
			log.Error("imageserver: resolve interface failed", "interface", name, "error", err)
			continue
		}
		targets = append(targets, listenTarget{iface: name, ip: ip})
	}
	return targets, nil
}

// startTargets binds one HTTP server for each target and returns the servers
// that bound. Not strict, a target that fails to bind is reported and
// skipped, so one unusable interface does not stop the others from serving;
// strict, the servers already bound are closed and the failure is the error.
// The caller MUST call Close on every returned server to stop its serve
// goroutine.
func startTargets(cfg imageConfig, zefsPath string, targets []listenTarget, log *slog.Logger, strict bool) ([]*http.Server, error) {
	servers := make([]*http.Server, 0, len(targets))
	for _, target := range targets {
		srv, err := imageServeTarget(cfg, zefsPath, target)
		if err != nil && strict {
			for _, bound := range servers {
				closeLogged(bound, log, "http server")
			}
			return nil, unboundImageInterface(target.iface, err)
		}
		if err != nil {
			log.Error("imageserver: listen failed",
				"interface", target.iface, "addr", listenAddr(target.ip, cfg.ListenPort), "error", err)
			continue
		}
		log.Info("imageserver: listening", "interface", target.iface, "addr", srv.Addr)
		servers = append(servers, srv)
	}
	return servers, nil
}

// unboundImageInterface is the reload error for a listen-interface the image
// server cannot resolve or bind. An empty name is the "no interface named"
// target, which binds every address of the host.
func unboundImageInterface(name string, err error) error {
	if name == "" {
		return fmt.Errorf("imageserver: cannot listen on every address: %w. "+
			"The image server does not take this config.\n"+
			"Fix: free the listen-port under service image-server, or choose another one", err)
	}
	return fmt.Errorf("imageserver: cannot listen on interface %q: %w. "+
		"The image server does not take this config.\n"+
		"Fix: name an interface that exists and has an IPv4 address in service image-server listen-interface, or remove %q from it", name, err, name)
}

// serveTarget binds one HTTP server on the target address and serves it in its
// own goroutine. The mux carries the target address, so the boot.ipxe script
// this listener serves names the address the client reached rather than another
// interface's. The caller MUST call Close on the returned server to stop the
// goroutine.
func serveTarget(cfg imageConfig, zefsPath string, target listenTarget) (*http.Server, error) {
	addr := listenAddr(target.ip, cfg.ListenPort)
	srv := &http.Server{
		Addr:              addr,
		Handler:           newMux(cfg, zefsPath, target.ip),
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      5 * time.Minute,
		MaxHeaderBytes:    1 << 16,
	}

	// Bind synchronously so a failed bind is reported honestly. The old code
	// logged "started" inside the serve goroutine and only reported a bind
	// failure afterwards, which masked the install-path failure where the
	// server never came up at all.
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", addr)
	if err != nil {
		return nil, err
	}

	// Serve, not ListenAndServe, so Addr is free to carry the address the
	// kernel gave us. It differs from the requested one when the port is 0.
	srv.Addr = ln.Addr().String()

	go func() {
		if serveErr := srv.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
			loggerPtr.Load().Error("imageserver: serve error", "addr", srv.Addr, "error", serveErr)
		}
	}()

	return srv, nil
}

// listenAddr joins a bind address and a port. An empty ip yields ":port", which
// binds every address of the host.
func listenAddr(ip string, port int) string {
	return net.JoinHostPort(ip, strconv.Itoa(port))
}

// resolveInterfaceIPv4 returns the first IPv4 address of the logical interface,
// resolved through the shared iface resolver so the image server bind address
// honors the os-name / mac-match selectors instead of assuming name == kernel
// device.
func resolveInterfaceIPv4(ifaceName string) (string, error) {
	addrs, err := iface.Addresses(ifaceName)
	if err != nil {
		// The iface backend may not be loaded -- the install/provision path
		// configures the interface directly via netlink without starting the
		// iface component. Fall back to a direct kernel lookup by the
		// configured name, which is the os device in that case.
		return interfaceIPv4Direct(ifaceName)
	}
	for _, a := range addrs {
		if a.Family == "ipv4" {
			return a.Address, nil
		}
	}
	return "", fmt.Errorf("interface %q: no IPv4 address", ifaceName)
}

// interfaceIPv4Direct returns the first IPv4 address of the named kernel
// interface via the standard library, bypassing the iface resolver. Used as a
// fallback when no iface backend is loaded (see resolveInterfaceIPv4).
func interfaceIPv4Direct(ifaceName string) (string, error) {
	ni, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return "", fmt.Errorf("interface %q: %w", ifaceName, err)
	}
	addrs, err := ni.Addrs()
	if err != nil {
		return "", fmt.Errorf("interface %q: %w", ifaceName, err)
	}
	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		if ip4 := ipNet.IP.To4(); ip4 != nil {
			return ip4.String(), nil
		}
	}
	return "", fmt.Errorf("interface %q: no IPv4 address", ifaceName)
}

type closer interface {
	Close() error
}

func closeLogged(c closer, log *slog.Logger, what string) {
	if err := c.Close(); err != nil {
		log.Debug("imageserver: close failed", "what", what, "error", err)
	}
}

// logServedImage reports the image that will actually be installed (the newest
// .img in the image directory, matching serveBootIPXE's latestImage) together
// with its build.json identity, so an operator can confirm at a glance that the
// latest build is being served rather than a stale or wrong image.
func logServedImage(log *slog.Logger, imageDir string) {
	if imageDir == "" {
		return
	}
	served, err := latestImage(imageDir)
	if err != nil {
		log.Warn("imageserver: no installable .img found", "image-directory", imageDir, "error", err)
		return
	}

	attrs := []any{"image", served}
	var m version.ImageInfo
	m, found := version.ReadManifestFile(filepath.Join(imageDir, "build.json"))
	if found {
		attrs = append(attrs,
			"appliance", m.Appliance,
			"built", m.Timestamp,
			"ze-version", m.ZeVersion,
			"arch", m.Arch,
			"sha256", m.SHA256)
		if m.Image != "" && m.Image != served {
			log.Warn("imageserver: build.json names a different image than the one being served",
				"manifest-image", m.Image, "serving", served)
		}
	} else {
		log.Warn("imageserver: no build.json next to image; cannot confirm build identity",
			"image-directory", imageDir)
	}
	log.Info("imageserver: image to install", attrs...)
}

func logAvailableFiles(log *slog.Logger, label, dir string) {
	if dir == "" {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Warn("imageserver: cannot list directory", "label", label, "dir", dir, "error", err)
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, infoErr := e.Info()
		if infoErr != nil {
			continue
		}
		log.Info("imageserver: serving file", "label", label, "name", e.Name(), "size", info.Size())
	}
}
