module github.com/ze-software/ze

go 1.26

// The stdlib patch floor, not a language floor. go1.26.5 carries eight
// vulnerabilities govulncheck reports as reachable from ze code: GO-2026-6218
// (net/url), GO-2026-6091 (html/template), GO-2026-6090 (crypto/tls),
// GO-2026-6089 and GO-2026-5026 (net/http), GO-2026-6088 (encoding/xml),
// GO-2026-5972 (encoding/asn1), GO-2026-5942 (net). All eight are fixed in
// go1.26.6. Every workflow builds with `go-version-file: go.mod` and
// actions/setup-go defaults to `check-latest: false`, so a bare `go 1.26` let
// the runner reuse its cached go1.26.5 and the scheduled govulncheck job went
// red. This line is what makes the patched stdlib the floor.
toolchain go1.26.6

require (
	charm.land/bubbles/v2 v2.1.1
	charm.land/bubbletea/v2 v2.0.8
	charm.land/lipgloss/v2 v2.0.6
	charm.land/wish/v2 v2.0.3
	github.com/beevik/ntp v1.5.0
	github.com/charmbracelet/colorprofile v0.4.3
	github.com/gaissmai/bart v0.29.0
	github.com/google/nftables v0.3.0
	github.com/insomniacslk/dhcp v0.0.0-20260719225207-c76316d4aa82
	github.com/miekg/dns v1.1.72
	github.com/muesli/reflow v0.3.0
	github.com/openconfig/goyang v1.6.3
	github.com/packetcap/go-pcap v0.0.0-20251215121130-f2cf9f991e7c
	github.com/prometheus/client_golang v1.24.1
	github.com/stretchr/testify v1.11.1
	github.com/vishvananda/netlink v1.3.1
	github.com/vishvananda/netns v0.0.5
	go.fd.io/govpp v0.13.0
	golang.org/x/crypto v0.55.0
	golang.org/x/net v0.58.0
	golang.org/x/sys v0.47.0
	golang.org/x/term v0.45.0
	golang.org/x/tools v0.48.0
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20241231184526-a9ab2273dd10
	google.golang.org/grpc v1.83.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.6.2
	google.golang.org/protobuf v1.36.12
)

require (
	charm.land/ssh v0.4.3
	github.com/a-h/templ v0.3.1020
	github.com/cilium/ebpf v0.22.0
	github.com/gokrazy/tools v0.0.0-20260703063348-3fe400c13246
	github.com/gokrazy/updater v0.0.0-20260620140544-0a84d8ab3878
	github.com/openconfig/gnmi v0.14.1
	github.com/sivchari/gomu v0.2.1
)

require (
	github.com/a-h/parse v0.0.0-20250122154542-74294addb73e // indirect
	github.com/andybalholm/brotli v1.1.0 // indirect
	github.com/antihax/optional v1.0.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/charmbracelet/x/xpty v0.1.4 // indirect
	github.com/cli/browser v1.3.0 // indirect
	github.com/donovanhide/eventsource v0.0.0-20210830082556-c59027999da0 // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/gokrazy/gokapi v0.0.0-20251205165548-0927bab199d4 // indirect
	github.com/gokrazy/internal v0.0.0-20260625065634-6994f9152c44 // indirect
	github.com/google/renameio/v2 v2.0.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/klauspost/compress v1.19.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.22 // indirect
	github.com/natefinch/atomic v1.0.1 // indirect
	github.com/pires/go-proxyproto v0.12.0 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	golang.org/x/crypto/x509roots/fallback v0.0.0-20260630172432-7626c5025624 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
)

require (
	charm.land/log/v2 v2.0.0 // indirect
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be // indirect
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/charmbracelet/keygen v0.5.4 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20260811164956-006e29f97886 // indirect
	github.com/charmbracelet/x/ansi v0.11.8 // indirect
	github.com/charmbracelet/x/conpty v0.2.0 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/creack/pty v1.1.24
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/ftrvxmtrx/fd v0.0.0-20150925145434-c6d800382fff // indirect
	github.com/go-logfmt/logfmt v0.6.1 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/lunixbochs/struc v0.0.0-20200521075829-a4cb8d33dbbe // indirect
	github.com/mattn/go-runewidth v0.0.24 // indirect
	github.com/mdlayher/genetlink v1.4.0
	github.com/mdlayher/netlink v1.11.2
	github.com/mdlayher/packet v1.1.2
	github.com/mdlayher/socket v0.6.0 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pierrec/lz4/v4 v4.1.14 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.70.1 // indirect
	github.com/prometheus/procfs v0.21.1
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/sirupsen/logrus v1.9.4
	github.com/u-root/uio v0.0.0-20230220225925-ffce2a382923 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	golang.org/x/exp v0.0.0-20260410095643-746e56fc9e2f // indirect
	golang.org/x/mod v0.39.0
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/telemetry v0.0.0-20260708182218-49f421fb7959 // indirect
	golang.org/x/text v0.41.0 // indirect
	golang.zx2c4.com/wireguard v0.0.0-20231211153847-12269c276173 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	gopkg.in/yaml.v3 v3.0.1
)
