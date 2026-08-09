// Design: docs/architecture/provisioning/image-server.md -- HTTP image/boot file serving

package imageserver

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

const (
	sshHost = "127.0.0.1"
	sshPort = "2222"
)

type imageHandler struct {
	imageDir   string
	bootDir    string
	zefsPath   string
	serverAddr string
	serverPort int
	rescueAuth string // salted argon2id of the rescue token; gates the installer rescue shell
}

func newMux(cfg imageConfig, zefsPath, serverAddr string) *http.ServeMux {
	h := &imageHandler{
		imageDir:   cfg.ImageDirectory,
		bootDir:    cfg.BootDirectory,
		zefsPath:   zefsPath,
		serverAddr: serverAddr,
		serverPort: cfg.ListenPort,
		rescueAuth: cfg.RescueAuth,
	}

	mux := http.NewServeMux()
	if cfg.ImageDirectory != "" {
		mux.HandleFunc("/install/image/", logRequest(h.serveImage))
	}
	if cfg.BootDirectory != "" {
		if serverAddr != "" {
			mux.HandleFunc("/install/boot/boot.ipxe", logRequest(h.serveBootIPXE))
		}
		mux.HandleFunc("/install/boot/", logRequest(h.serveBoot))
	}
	if zefsPath != "" {
		mux.HandleFunc("/install/database.zefs", logRequest(h.serveZefs))
	}
	return mux
}

// Large downloads (the multi-GB image especially) stream for a long time.
// progressWriter wraps the response writer so an operator watching the install
// sees the transfer move -- start, periodic progress, and a final throughput
// line -- instead of silence followed by a single status line at the end.
// Vars (not consts) so tests can lower the threshold.
var (
	progressThreshold int64 = 8 << 20 // only files at least this big get progress logging
	progressInterval        = 3 * time.Second
)

type progressWriter struct {
	http.ResponseWriter
	log     *slog.Logger
	file    string
	remote  string
	total   int64
	written int64
	start   time.Time
	lastLog time.Time
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.ResponseWriter.Write(b)
	p.written += int64(n)
	if now := time.Now(); now.Sub(p.lastLog) >= progressInterval {
		p.lastLog = now
		p.emit("imageserver: sending")
	}
	return n, err
}

// emit logs one progress line with cumulative throughput and percent complete.
func (p *progressWriter) emit(msg string) {
	mbps := 0
	if elapsed := time.Since(p.start).Seconds(); elapsed > 0 {
		mbps = int(float64(p.written) / elapsed / (1 << 20))
	}
	pct := 0
	if p.total > 0 {
		pct = int(p.written * 100 / p.total)
	}
	p.log.Info(msg,
		"file", p.file, "remote", p.remote,
		"sent", p.written, "total", p.total, "percent", pct, "mbps", mbps)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func logRequest(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next(rec, r)
		log := loggerPtr.Load()
		log.Info("imageserver: request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr, "status", rec.status)
	}
}

func buildZefsDB(dir, username, passwordHash string) (string, error) {
	path := filepath.Join(dir, "database.zefs")
	store, err := zefs.Create(path)
	if err != nil {
		return "", err
	}

	entries := []struct{ key, value string }{
		{zefs.KeySSHUsername.Key(sshHost, sshPort), username},
		{zefs.KeySSHPassword.Key(sshHost, sshPort), passwordHash},
		{zefs.KeyLocalAdminUsername.Pattern, username},
		{zefs.KeyLocalAdminPassword.Pattern, passwordHash},
		{zefs.KeySSHDefault.Pattern, sshHost + "/" + sshPort},
	}

	for _, e := range entries {
		if writeErr := store.WriteFile(e.key, []byte(e.value), 0); writeErr != nil {
			// Best-effort cleanup; return the write error (the root cause),
			// not a secondary close/remove failure that would mask it.
			store.Close()   //nolint:errcheck // cleanup; writeErr is the real error
			os.Remove(path) //nolint:errcheck // cleanup; writeErr is the real error
			return "", writeErr
		}
	}

	if err := store.Close(); err != nil {
		os.Remove(path) //nolint:errcheck // cleanup; err is the real error
		return "", err
	}

	return path, nil
}

func (h *imageHandler) serveImage(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/install/image/")
	h.serveFromDir(w, r, h.imageDir, name)
}

func (h *imageHandler) serveBoot(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/install/boot/")
	h.serveFromDir(w, r, h.bootDir, name)
}

func (h *imageHandler) serveBootIPXE(w http.ResponseWriter, r *http.Request) {
	if _, err := os.Stat(filepath.Join(h.bootDir, "boot.ipxe")); err == nil {
		h.serveFromDir(w, r, h.bootDir, "boot.ipxe")
		return
	}

	imgName, err := latestImage(h.imageDir)
	if err != nil {
		http.Error(w, "no image found in image-directory", http.StatusServiceUnavailable)
		return
	}

	tb := textbuf.Get()
	defer tb.Release()

	baseURL := tb.Reset().Str("http://").Str(h.serverAddr).String()
	if h.serverPort != 0 && h.serverPort != 80 {
		baseURL = tb.Reset().Str("http://").Str(h.serverAddr).Str(":").Int(int64(h.serverPort)).String()
	}

	var portArg string
	if h.serverPort != 0 && h.serverPort != 80 {
		portArg = tb.Reset().Str(" ze.port=").Int(int64(h.serverPort)).String()
	}

	// Pass the salted argon2id of the rescue token so the installer can gate its
	// rescue shell (ze.rescue-auth). This script is served unauthenticated over
	// plain HTTP, so the value is public by construction: it commits only to a
	// dedicated rescue token, never to the admin password. Omitted when unset ->
	// the installer fails closed (prints the error and reboots, no shell).
	var authArg string
	if h.rescueAuth != "" {
		authArg = tb.Reset().Str(" ze.rescue-auth=").Str(h.rescueAuth).String()
	}

	// Select the console set on the client via iPXE's ${buildarch}: one server
	// PXE-boots heterogeneous clients, so the arch is known to iPXE, not to us.
	// ttyAMA0 is the ARM PL011 UART and never registers on x86; left on an x86
	// cmdline it can leave /dev/console (and the installer's userspace stdio)
	// pointing at a dead device. Give x86 only tty0+ttyS0; keep the full set
	// for arm64. The iseq && ... || ... form always runs exactly one branch, so
	// the script never aborts when buildarch is not arm64.
	script := tb.Reset().
		Str("#!ipxe\n").
		Str("iseq ${buildarch} arm64 && set zeconsole console=tty0 console=ttyS0,115200n8 console=ttyAMA0,115200n8 || set zeconsole console=tty0 console=ttyS0,115200n8\n").
		Str("kernel ").Str(baseURL).Str("/install/boot/vmlinuz ze.server=").Str(h.serverAddr).
		Str(" ze.image=").Str(imgName).Str(portArg).
		// loglevel=8 + earlycon match the runtime kernel's KernelExtraArgs so the
		// installer kernel shows all boot/driver/console messages from the earliest
		// possible moment. Without them the installer kernel defaults to the quiet
		// loglevel and a client whose framebuffer/serial console hands over late
		// shows a blank screen through the entire install.
		Str(" ip=dhcp ze.mac=${mac}").Str(authArg).Str(" loglevel=8 earlycon panic=-1 ${zeconsole}\n").
		Str("initrd ").Str(baseURL).Str("/install/boot/initrd.img.gz\n").
		Str("boot\n").
		String()

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(script)) //nolint:errcheck // best-effort HTTP write
}

func latestImage(dir string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("no image directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var imgs []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".img") && !strings.HasPrefix(e.Name(), ".") {
			imgs = append(imgs, e.Name())
		}
	}
	if len(imgs) == 0 {
		return "", fmt.Errorf("no .img files")
	}
	sort.Strings(imgs)
	return imgs[len(imgs)-1], nil
}

// serveZefs serves the pre-provisioned zefs database (SSH username + password
// HASH, no plaintext) to the PXE installer. Served without authentication by
// design: PXE provisioning runs on a trusted install network where the kernel,
// initrd, and disk image are themselves fetched unauthenticated, so the trust
// boundary is the network, not this endpoint.
func (h *imageHandler) serveZefs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	http.ServeFile(w, r, h.zefsPath)
}

func (h *imageHandler) serveFromDir(w http.ResponseWriter, r *http.Request, dir, name string) {
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		http.NotFound(w, r)
		return
	}
	if name == "." || name == ".." || strings.ContainsRune(name, 0) {
		http.NotFound(w, r)
		return
	}

	cleaned := filepath.Clean(name)
	if cleaned != name {
		http.NotFound(w, r)
		return
	}

	full := filepath.Join(dir, cleaned)

	// Log progress for large, full-GET downloads (the install image especially).
	// Range requests are left to plain ServeFile so the logged total stays
	// meaningful; the on-device installer fetches the image with a single GET.
	if info, statErr := os.Stat(full); statErr == nil && info.Size() >= progressThreshold &&
		r.Method == http.MethodGet && r.Header.Get("Range") == "" {
		log := loggerPtr.Load()
		pw := &progressWriter{
			ResponseWriter: w, log: log, file: name, remote: r.RemoteAddr,
			total: info.Size(), start: time.Now(), lastLog: time.Now(),
		}
		log.Info("imageserver: sending", "file", name, "remote", r.RemoteAddr, "total", info.Size())
		http.ServeFile(pw, r, full)
		pw.emit("imageserver: sent")
		return
	}

	http.ServeFile(w, r, full)
}
