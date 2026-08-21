// Design: docs/architecture/testing/runner-architecture.md — .wb server kinds

package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/ze-software/ze/internal/component/lg"
	"github.com/ze-software/ze/internal/component/plugin"
)

// errLGEngineUnavailable is what the stubbed dispatcher answers with. The
// looking glass turns a dispatch failure into this text, in the page banner and
// in the peer stream, so the browser reads the words this error carries.
var errLGEngineUnavailable = errors.New("bgp engine unavailable")

// cmdLG serves the REAL looking glass with an engine that always fails.
//
// It exists because a daemon cannot be asked for this state. The looking glass
// dispatches in process, so `show bgp` answers an empty peer list when
// no BGP is configured; the engine-unavailable path needs a dispatcher that
// returns an error, and only a test binary can inject one. Everything else here
// is production code: lg.NewLGServer, the real routes, the real templates and
// the real event stream.
//
// The .wb suite starts it for `option=server:kind=lg-no-engine`.
func cmdLG(args []string) int {
	fs := flag.NewFlagSet("lg", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	listen := fs.String("listen", "127.0.0.1:0", "address the looking glass binds (host:port)")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	srv, err := lg.NewLGServer(lg.LGConfig{
		ListenAddrs: []string{*listen},
		Dispatch: func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
			return nil, errLGEngineUnavailable
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "looking glass:", err) //nolint:errcheck // output

		return 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	defer signal.Stop(sigCh)

	go func() {
		<-sigCh
		cancel()

		if shutErr := srv.Shutdown(context.Background()); shutErr != nil {
			fmt.Fprintln(os.Stderr, "looking glass shutdown:", shutErr) //nolint:errcheck // output
		}
	}()

	if serveErr := srv.ListenAndServe(ctx); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, "looking glass:", serveErr) //nolint:errcheck // output

		return 1
	}

	return 0
}
