// Design: docs/guide/environment-variables.md -- ze.pprof starts the profiling server
// Related: ../../../cmd/ze/pprof.go -- startPprof, the server test/ui/env-pprof-serves.ci reaches

package fixture

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

func init() {
	Register("ui/pprof-probe", pprofProbe)
}

// pprofIndexNeedle is a line net/http/pprof writes on its index page and no
// other HTTP server Ze runs would answer.
const pprofIndexNeedle = "Types of profiles available"

// pprofProbe fetches http://127.0.0.1:<port>/debug/pprof/ until the pprof
// index answers, and fails when it never does within the poll budget.
func pprofProbe(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: ui/pprof-probe <port>")
	}
	url := "http://" + net.JoinHostPort("127.0.0.1", args[0]) + "/debug/pprof/"
	client := &http.Client{Timeout: time.Second}
	var lastErr error
	answered := Poll(ctx, 60, 250*time.Millisecond, func() bool {
		lastErr = pprofFetchIndex(ctx, client, url)
		return lastErr == nil
	})
	if !answered {
		return fmt.Errorf("pprof index never answered at %s: %w", url, lastErr)
	}
	fmt.Fprintln(os.Stdout, "pprof index answered at "+url) //nolint:errcheck // fixture progress output
	return nil
}

func pprofFetchIndex(ctx context.Context, client *http.Client, url string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close() //nolint:errcheck // read-only probe
	body, err := io.ReadAll(io.LimitReader(response.Body, 64*1024))
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", response.StatusCode)
	}
	if !strings.Contains(string(body), pprofIndexNeedle) {
		return errors.New("the answer is not the pprof index")
	}
	return nil
}
