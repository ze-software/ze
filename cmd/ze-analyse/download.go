// Design: (none -- research/analysis tool)
//
// Download MRT RIB dumps and BGP4MP updates from RIPE RIS and RouteViews.
// Replaces test/internet/download.sh with a Go implementation.
package main

import (
	"compress/bzip2"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func runDownload(args []string) int {
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	outDir := fs.String("o", "test/internet", "Output directory for downloaded files")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `ze-analyse download -- fetch MRT data from public BGP collectors

Downloads RIB dumps and UPDATE streams from RIPE RIS (rrc00) and RouteViews.
Files are saved as .gz for Go stdlib compatibility.

Usage:
  ze-analyse download [options] [YYYYMMDD] [HHMM]

Arguments:
  YYYYMMDD    Date for data files (default: today)
  HHMM        Time slot (default: 0000). RIPE: 5-min intervals. RouteViews: 15-min.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
What gets downloaded:
  latest-bview.gz                  RIPE RIS full routing table snapshot (~400 MB)
  ripe-updates.YYYYMMDD.HHMM.gz   RIPE RIS live BGP4MP updates (~5 MB per 5-min file)
  rib.YYYYMMDD.HHMM.gz            RouteViews full table (~100 MB)
  rv-updates.YYYYMMDD.HHMM.gz     RouteViews BGP4MP updates (~2 MB per 15-min file)

Examples:
  ze-analyse download                     # latest RIB + today's updates at 00:00
  ze-analyse download 20260324            # specific date
  ze-analyse download 20260324 1200       # specific date and time
`)
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	now := time.Now().UTC()
	date := now.Format("20060102")
	timeSlot := "0000"

	if fs.NArg() >= 1 {
		date = fs.Arg(0)
	}
	if fs.NArg() >= 2 {
		timeSlot = fs.Arg(1)
	}

	if len(date) != 8 || !isAllDigits(date) {
		fmt.Fprintf(os.Stderr, "error: date must be YYYYMMDD (digits only), got %q\n", date)
		return 1
	}
	if len(timeSlot) != 4 || !isAllDigits(timeSlot) {
		fmt.Fprintf(os.Stderr, "error: time must be HHMM (digits only), got %q\n", timeSlot)
		return 1
	}

	month := date[:4] + "." + date[4:6]

	if err := os.MkdirAll(*outDir, 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "error: creating output dir: %v\n", err)
		return 1
	}

	type dlTask struct {
		name string
		url  string
		out  string
	}

	tasks := []dlTask{
		{
			name: "RIPE RIS latest RIB",
			url:  "https://data.ris.ripe.net/rrc00/latest-bview.gz",
			out:  filepath.Join(*outDir, "latest-bview.gz"),
		},
		{
			name: fmt.Sprintf("RIPE RIS updates %s.%s", date, timeSlot),
			url:  fmt.Sprintf("https://data.ris.ripe.net/rrc00/%s/updates.%s.%s.gz", month, date, timeSlot),
			out:  filepath.Join(*outDir, fmt.Sprintf("ripe-updates.%s.%s.gz", date, timeSlot)),
		},
		{
			name: fmt.Sprintf("RouteViews RIB %s.%s", date, timeSlot),
			url:  fmt.Sprintf("https://archive.routeviews.org/bgpdata/%s/RIBS/rib.%s.%s.bz2", month, date, timeSlot),
			out:  filepath.Join(*outDir, fmt.Sprintf("rib.%s.%s.gz", date, timeSlot)),
		},
		{
			name: fmt.Sprintf("RouteViews updates %s.%s", date, timeSlot),
			url:  fmt.Sprintf("https://archive.routeviews.org/bgpdata/%s/UPDATES/updates.%s.%s.bz2", month, date, timeSlot),
			out:  filepath.Join(*outDir, fmt.Sprintf("rv-updates.%s.%s.gz", date, timeSlot)),
		},
	}

	failed := 0
	for _, t := range tasks {
		fmt.Fprintf(os.Stderr, "Downloading %s...\n", t.name)
		if err := downloadFile(t.url, t.out); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: %v\n", err)
			failed++
			continue
		}
		fi, err := os.Stat(t.out)
		if err == nil {
			fmt.Fprintf(os.Stderr, "  saved: %s (%s)\n", t.out, formatBytes(uint64(fi.Size()))) //nolint:gosec // file size is positive
		}
	}

	if failed == len(tasks) {
		fmt.Fprintf(os.Stderr, "error: all downloads failed\n")
		return 1
	}

	fmt.Fprintf(os.Stderr, "\nDone. Use 'ze-analyse density %s/ripe-updates.*.gz' to analyze.\n", *outDir)
	return 0
}

// downloadFile fetches a URL and saves to disk.
// If the URL is .bz2, it converts to .gz on the fly.
func downloadFile(url, outPath string) error {
	resp, err := http.Get(url) //nolint:gosec,noctx // CLI tool, URL from hardcoded templates
	if err != nil {
		return fmt.Errorf("HTTP GET %s: %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close on HTTP response

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	out, err := os.Create(outPath) //nolint:gosec // outPath from CLI args
	if err != nil {
		return fmt.Errorf("creating %s: %w", outPath, err)
	}

	// If source is .gz, save directly. If .bz2, we need to recompress.
	// For simplicity, save the raw response -- our MRT reader handles both formats.
	// However, the output filename is always .gz, so if source is .bz2 we convert.
	var writeErr error
	if strings.HasSuffix(url, ".bz2") {
		// bz2 -> gz conversion: read bz2, compress as gzip.
		// Note: we import compress/bzip2 in mrt.go; use it here via the reader.
		writeErr = convertBZ2ToGZ(resp.Body, out)
	} else {
		_, writeErr = io.Copy(out, resp.Body)
	}

	if closeErr := out.Close(); closeErr != nil && writeErr == nil {
		writeErr = closeErr
	}

	if writeErr != nil {
		// Clean up partial file.
		os.Remove(outPath) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("writing %s: %w", outPath, writeErr)
	}

	return nil
}

// convertBZ2ToGZ reads bzip2 data and writes gzip-compressed output.
func convertBZ2ToGZ(src io.Reader, dst io.Writer) error {
	// Use compress/bzip2 from stdlib.
	bzReader := bzip2.NewReader(src)

	gzWriter, err := gzip.NewWriterLevel(dst, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("creating gzip writer: %w", err)
	}

	if _, err := io.Copy(gzWriter, bzReader); err != nil { //nolint:gosec // CLI tool, input from trusted sources
		return fmt.Errorf("converting bz2 to gz: %w", err)
	}

	return gzWriter.Close()
}
