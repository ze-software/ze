package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func cmdHTTPGet(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "http-get: exactly one URL is required")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, args[0], nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "http-get: %v\n", err)
		return 1
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		fmt.Fprintf(os.Stderr, "http-get: %v\n", err)
		return 1
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		fmt.Fprintf(os.Stderr, "http-get: %s\n", response.Status)
		return 1
	}
	if _, err := io.Copy(os.Stdout, response.Body); err != nil {
		fmt.Fprintf(os.Stderr, "http-get: copy response: %v\n", err)
		return 1
	}
	return 0
}
