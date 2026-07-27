// Design: docs/architecture/mrt.md — file reader with decompression

package mrt

import (
	"bufio"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/cliio"
)

// Handler receives decoded MRT records. Set callbacks for the record types
// you want; nil callbacks are skipped. Return a non-nil error to stop iteration.
type Handler struct {
	OnHeader      func(h Header, microsecond uint32, data []byte) error
	OnPeerIndex   func(h Header, pit *PeerIndexTable) error
	OnRIB         func(h Header, r *RIBRecord) error
	OnRIBGeneric  func(h Header, r *RIBGenericRecord) error
	OnGeoPeer     func(h Header, g *GeoPeerTable) error
	OnMessage     func(h Header, microsecond uint32, m *MessageRecord) error
	OnStateChange func(h Header, microsecond uint32, s *StateChangeRecord) error
	OnTableDump   func(h Header, t *TableDumpRecord) error
}

// ErrRecordTooLarge is returned when a record's Length exceeds MaxRecordLen.
var ErrRecordTooLarge = errors.New("mrt: record length exceeds maximum")

// ReadFile opens an MRT file, auto-detects compression by extension
// (.gz, .bz2), and iterates records through handler callbacks.
func ReadFile(filename string, handler *Handler) error {
	rc, err := openReader(filename)
	if err != nil {
		return err
	}
	readErr := ReadFrom(rc, handler)
	closeErr := rc.Close()
	if readErr != nil {
		return readErr
	}
	return closeErr
}

// ReadFrom iterates MRT records from any reader, dispatching to handler callbacks.
func ReadFrom(r io.Reader, handler *Handler) error {
	br := bufio.NewReaderSize(r, 64*1024)
	return readRecords(br, handler)
}

func openReader(filename string) (io.ReadCloser, error) {
	if strings.HasPrefix(filename, "http://") || strings.HasPrefix(filename, "https://") {
		return openHTTPReader(filename)
	}
	// Both "-" (stdin) and a real path go through cliio: the "-" token is resolved
	// to stdin and no raw os.Open on a CLI-supplied path escapes the shared helper
	// (enforced by scripts/checks/cli_dash_stdio.go). filename IS user input.
	rc, err := cliio.OpenReader(filename)
	if err != nil {
		return nil, err
	}
	if cliio.IsStdin(filename) {
		// Stdin has no filename extension: sniff compression by leading magic
		// bytes so a gzipped/bzip2'd pipe is not misread as raw (R-4).
		return SniffDecompress(rc)
	}
	return wrapByExtension(rc, strings.ToLower(filename))
}

// wrapByExtension decompresses rc according to the filename suffix (.gz, .bz2),
// preserving the pre-existing real-path behavior. The returned ReadCloser owns
// rc and closes it.
func wrapByExtension(rc io.ReadCloser, lower string) (io.ReadCloser, error) {
	switch {
	case strings.HasSuffix(lower, ".gz"):
		gr, err := gzip.NewReader(rc)
		if err != nil {
			if cerr := rc.Close(); cerr != nil {
				return nil, fmt.Errorf("gzip init: %w (close: %w)", err, cerr)
			}
			return nil, err
		}
		return &decompressCloser{r: gr, under: rc, inner: gr}, nil
	case strings.HasSuffix(lower, ".bz2"):
		return &readerCloser{r: bzip2.NewReader(rc), cls: rc}, nil
	default:
		return rc, nil
	}
}

// SniffDecompress wraps rc with a gzip or bzip2 decompressor when the stream's
// leading magic bytes indicate compression (gzip 1f 8b, bzip2 "BZh"), otherwise
// returns rc reading raw. Only the few peeked bytes are buffered, so the stream
// stays unbuffered for multi-GB inputs. Used for stdin ("-"), which has no
// filename extension to sniff. The returned ReadCloser owns rc and closes it.
func SniffDecompress(rc io.ReadCloser) (io.ReadCloser, error) {
	br := bufio.NewReaderSize(rc, 64*1024)
	magic, err := br.Peek(3)
	// A stream shorter than 3 bytes yields io.EOF/ErrUnexpectedEOF from Peek with
	// the available bytes intact; that is fine (a tiny stream is not compressed).
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		if cerr := rc.Close(); cerr != nil {
			return nil, fmt.Errorf("sniff: %w (close: %w)", err, cerr)
		}
		return nil, err
	}
	switch {
	case len(magic) >= 2 && magic[0] == 0x1f && magic[1] == 0x8b:
		gr, gerr := gzip.NewReader(br)
		if gerr != nil {
			if cerr := rc.Close(); cerr != nil {
				return nil, fmt.Errorf("gzip init: %w (close: %w)", gerr, cerr)
			}
			return nil, gerr
		}
		return &decompressCloser{r: gr, under: rc, inner: gr}, nil
	case len(magic) >= 3 && magic[0] == 'B' && magic[1] == 'Z' && magic[2] == 'h':
		return &decompressCloser{r: bzip2.NewReader(br), under: rc}, nil
	default:
		return &decompressCloser{r: br, under: rc}, nil
	}
}

// decompressCloser reads from r (raw buffered reader or a decompressor over the
// buffered reader) and closes the inner decompressor (if any) then the
// underlying source.
type decompressCloser struct {
	r     io.Reader
	under io.Closer
	inner io.Closer // gzip reader; nil for bzip2/raw
}

func (d *decompressCloser) Read(p []byte) (int, error) { return d.r.Read(p) }
func (d *decompressCloser) Close() error {
	var firstErr error
	if d.inner != nil {
		if err := d.inner.Close(); err != nil {
			firstErr = err
		}
	}
	if err := d.under.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func openHTTPReader(url string) (io.ReadCloser, error) {
	transport := &http.Transport{
		ResponseHeaderTimeout: 60 * time.Second,
	}
	client := &http.Client{Transport: transport}
	resp, err := client.Get(url) //nolint:gosec,noctx // CLI tool, URL from user args
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("mrt: HTTP %d for %s", resp.StatusCode, url)
	}
	lower := strings.ToLower(url)
	switch {
	case strings.HasSuffix(lower, ".gz"):
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			_ = resp.Body.Close()
			return nil, err
		}
		return &gzipHTTPCloser{gz: gr, body: resp.Body}, nil
	case strings.HasSuffix(lower, ".bz2"):
		return &readerCloser{r: bzip2.NewReader(resp.Body), cls: resp.Body}, nil
	default:
		return resp.Body, nil
	}
}

type gzipHTTPCloser struct {
	gz   *gzip.Reader
	body io.ReadCloser
}

func (g *gzipHTTPCloser) Read(p []byte) (int, error) { return g.gz.Read(p) }
func (g *gzipHTTPCloser) Close() error {
	gzErr := g.gz.Close()
	bErr := g.body.Close()
	if gzErr != nil {
		return gzErr
	}
	return bErr
}

type readerCloser struct {
	r   io.Reader
	cls io.Closer
}

func (b *readerCloser) Read(p []byte) (int, error) { return b.r.Read(p) }
func (b *readerCloser) Close() error               { return b.cls.Close() }

func readRecords(r io.Reader, handler *Handler) error {
	var hdrBuf [CommonHeaderLen]byte
	// Record ordinal, 1-based, counting every record read from this stream.
	// It is the only handle a user has on WHICH record failed: a decode error
	// carries an offset inside the record's own fields, which on a multi-GB
	// dump locates nothing (ai/rules/error-messages.md, "what to do next").
	var ordinal uint64
	for {
		_, err := io.ReadFull(r, hdrBuf[:])
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil
			}
			return fmt.Errorf("mrt: reading the header of record %d: %w", ordinal+1, err)
		}
		ordinal++

		h, err := DecodeHeader(hdrBuf[:])
		if err != nil {
			return fmt.Errorf("mrt: record %d header: %w", ordinal, err)
		}

		if h.Length > MaxRecordLen {
			return fmt.Errorf("%w: %d bytes (record %d, type %d subtype %d, timestamp %d)",
				ErrRecordTooLarge, h.Length, ordinal, h.Type, h.Subtype, h.Timestamp)
		}

		data := make([]byte, h.Length)
		if _, err := io.ReadFull(r, data); err != nil {
			return fmt.Errorf("mrt: truncated record %d (type %d subtype %d, timestamp %d): %w",
				ordinal, h.Type, h.Subtype, h.Timestamp, err)
		}

		var microsecond uint32
		msgData := data
		if isETType(h.Type) && len(data) >= ExtTimestampLen {
			microsecond, _ = DecodeMicrosecond(data)
			msgData = data[ExtTimestampLen:]
		}

		if handler.OnHeader != nil {
			if err := handler.OnHeader(h, microsecond, data); err != nil {
				return fmt.Errorf("mrt: record %d (type %d subtype %d, timestamp %d): %w",
					ordinal, h.Type, h.Subtype, h.Timestamp, err)
			}
		}

		if err := dispatch(h, microsecond, msgData, handler); err != nil {
			return fmt.Errorf("mrt: record %d (type %d subtype %d, timestamp %d): %w",
				ordinal, h.Type, h.Subtype, h.Timestamp, err)
		}
	}
}

func isETType(typ uint16) bool {
	return typ == TypeBGP4MPET || typ == TypeISISET || typ == TypeOSPFv3ET
}

func dispatch(h Header, usec uint32, data []byte, handler *Handler) error {
	switch h.Type {
	case TypeTableDumpV2:
		return dispatchTDV2(h, data, handler)
	case TypeBGP4MP, TypeBGP4MPET:
		return dispatchBGP4MP(h, usec, data, handler)
	case TypeTableDump:
		return dispatchTD(h, data, handler)
	}
	return nil
}

func dispatchTDV2(h Header, data []byte, handler *Handler) error {
	switch h.Subtype {
	case TDV2PeerIndexTable:
		if handler.OnPeerIndex == nil {
			return nil
		}
		pit, err := DecodePeerIndexTable(data)
		if err != nil {
			return err
		}
		return handler.OnPeerIndex(h, pit)

	case TDV2RIBIPv4Unicast, TDV2RIBIPv4Multicast,
		TDV2RIBIPv6Unicast, TDV2RIBIPv6Multicast,
		TDV2RIBIPv4UnicastAP, TDV2RIBIPv4MulticastAP,
		TDV2RIBIPv6UnicastAP, TDV2RIBIPv6MulticastAP:
		if handler.OnRIB == nil {
			return nil
		}
		rec, err := DecodeRIBRecord(h.Subtype, data)
		if err != nil {
			return err
		}
		return handler.OnRIB(h, rec)

	case TDV2RIBGeneric, TDV2RIBGenericAP:
		if handler.OnRIBGeneric == nil {
			return nil
		}
		rec, err := DecodeRIBGenericRecord(h.Subtype, data)
		if err != nil {
			return err
		}
		return handler.OnRIBGeneric(h, rec)

	case TDV2GeoPeerTable:
		if handler.OnGeoPeer == nil {
			return nil
		}
		g, err := DecodeGeoPeerTable(data)
		if err != nil {
			return err
		}
		return handler.OnGeoPeer(h, g)
	}
	return nil
}

func dispatchBGP4MP(h Header, usec uint32, data []byte, handler *Handler) error {
	if IsStateChangeSubtype(h.Subtype) {
		if handler.OnStateChange == nil {
			return nil
		}
		sc, err := DecodeBGP4MPStateChange(h.Subtype, data)
		if err != nil {
			return err
		}
		return handler.OnStateChange(h, usec, sc)
	}

	switch h.Subtype {
	case BGP4MPMessage, BGP4MPMessageAS4,
		BGP4MPMessageLocal, BGP4MPMessageAS4Local,
		BGP4MPMessageAP, BGP4MPMessageAS4AP,
		BGP4MPMessageLocalAP, BGP4MPMessageAS4LocalAP:
		if handler.OnMessage == nil {
			return nil
		}
		msg, err := DecodeBGP4MPMessage(h.Subtype, data)
		if err != nil {
			return err
		}
		return handler.OnMessage(h, usec, msg)
	}
	return nil
}

func dispatchTD(h Header, data []byte, handler *Handler) error {
	if handler.OnTableDump == nil {
		return nil
	}
	rec, err := DecodeTableDump(h.Subtype, data)
	if err != nil {
		return err
	}
	return handler.OnTableDump(h, rec)
}
