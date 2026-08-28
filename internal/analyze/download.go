// Design: (none -- research/analysis tool)
//
// Download MRT RIB dumps and BGP4MP updates from RIPE RIS and RouteViews.
package analyze

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

	"github.com/ze-software/ze/internal/core/textbuf"
)

func runDownload(args []string) int {
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	outDir := fs.String("o", "test/internet", "Output directory for downloaded files")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `ze-analyze download -- fetch MRT data from public BGP collectors

Downloads RIB dumps and UPDATE streams from RIPE RIS (rrc00) and RouteViews.
Files are saved as .gz for Go stdlib compatibility.

Usage:
  ze-analyze download [options] [YYYYMMDD] [HHMM]

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
  ze-analyze download                     # latest RIB + today's updates at 00:00
  ze-analyze download 20260324            # specific date
  ze-analyze download 20260324 1200       # specific date and time
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

	var tb textbuf.Buffer
	month := tb.Str(date[:4]).Byte('.').Str(date[4:6]).String()

	if err := os.MkdirAll(*outDir, 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "error: creating output dir: %v\n", err)
		return 1
	}

	type dlTask struct {
		name     string
		url      string
		out      string
		required bool
	}

	tasks := []dlTask{
		{
			name:     "RIPE RIS latest RIB",
			url:      "https://data.ris.ripe.net/rrc00/latest-bview.gz",
			out:      filepath.Join(*outDir, "latest-bview.gz"),
			required: true,
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

	for _, task := range tasks {
		fmt.Fprintf(os.Stderr, "Downloading %s...\n", task.name)
		if err := downloadFile(task.url, task.out); err != nil {
			if task.required {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			fmt.Fprintf(os.Stderr, "  warning: %v\n", err)
			continue
		}
		fi, err := os.Stat(task.out)
		if err == nil {
			fmt.Fprintf(os.Stderr, "  saved: %s (%s)\n", task.out, formatBytes(uint64(fi.Size()))) //nolint:gosec // file size is positive
		}
	}

	fmt.Fprintf(os.Stderr, "\nDone. Use 'ze-analyze density %s/ripe-updates.*.gz' to analyze.\n", *outDir)
	return 0
}

// downloadFile fetches a gzip or bzip2 source and recompresses it with gzip's
// best compression level. Recompression both validates the transfer and keeps
// every output compatible with Go's gzip reader.
func downloadFile(url, outPath string) error {
	resp, err := http.Get(url) //nolint:gosec,noctx // CLI tool, URL from hardcoded templates
	if err != nil {
		return fmt.Errorf("HTTP GET %s: %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close on HTTP response

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	var source io.Reader
	var sourceCloser io.Closer
	switch {
	case strings.HasSuffix(url, ".bz2"):
		source = bzip2.NewReader(resp.Body)
	case strings.HasSuffix(url, ".gz"):
		gzReader, gzErr := gzip.NewReader(resp.Body)
		if gzErr != nil {
			return fmt.Errorf("reading gzip response from %s: %w", url, gzErr)
		}
		source = gzReader
		sourceCloser = gzReader
	default:
		return fmt.Errorf("unsupported compression for %s", url)
	}
	if sourceCloser != nil {
		defer sourceCloser.Close() //nolint:errcheck // decoder close has no additional validation
	}

	out, err := os.Create(outPath) //nolint:gosec // outPath from CLI args
	if err != nil {
		return fmt.Errorf("creating %s: %w", outPath, err)
	}

	writeErr := compressToGZ(source, out)
	if closeErr := out.Close(); closeErr != nil && writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		os.Remove(outPath) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("writing %s: %w", outPath, writeErr)
	}
	return nil
}

func compressToGZ(src io.Reader, dst io.Writer) error {
	gzWriter, err := gzip.NewWriterLevel(dst, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("creating gzip writer: %w", err)
	}
	_, copyErr := io.Copy(gzWriter, src)
	closeErr := gzWriter.Close()
	if copyErr != nil {
		return fmt.Errorf("compressing download: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("closing gzip writer: %w", closeErr)
	}
	return nil
}
