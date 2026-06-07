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
	"os"
	"strings"
	"time"
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
	cleaned := filename        // caller-controlled path; no user input
	f, err := os.Open(cleaned) //nolint:gosec // path is from CLI args, not user input
	if err != nil {
		return nil, err
	}
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".gz"):
		gr, err := gzip.NewReader(f)
		if err != nil {
			if cerr := f.Close(); cerr != nil {
				return nil, fmt.Errorf("gzip init: %w (close: %w)", err, cerr)
			}
			return nil, err
		}
		return &gzipCloser{gz: gr, file: f}, nil
	case strings.HasSuffix(lower, ".bz2"):
		return &readerCloser{r: bzip2.NewReader(f), cls: f}, nil
	default:
		return f, nil
	}
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

type gzipCloser struct {
	gz   *gzip.Reader
	file *os.File
}

func (g *gzipCloser) Read(p []byte) (int, error) { return g.gz.Read(p) }
func (g *gzipCloser) Close() error {
	gzErr := g.gz.Close()
	fErr := g.file.Close()
	if gzErr != nil {
		return gzErr
	}
	return fErr
}

type readerCloser struct {
	r   io.Reader
	cls io.Closer
}

func (b *readerCloser) Read(p []byte) (int, error) { return b.r.Read(p) }
func (b *readerCloser) Close() error               { return b.cls.Close() }

func readRecords(r io.Reader, handler *Handler) error {
	var hdrBuf [CommonHeaderLen]byte
	for {
		_, err := io.ReadFull(r, hdrBuf[:])
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil
			}
			return err
		}

		h, err := DecodeHeader(hdrBuf[:])
		if err != nil {
			return err
		}

		if h.Length > MaxRecordLen {
			return fmt.Errorf("%w: %d bytes (type %d subtype %d)", ErrRecordTooLarge, h.Length, h.Type, h.Subtype)
		}

		data := make([]byte, h.Length)
		if _, err := io.ReadFull(r, data); err != nil {
			return fmt.Errorf("mrt: truncated record (type %d subtype %d): %w", h.Type, h.Subtype, err)
		}

		var microsecond uint32
		msgData := data
		if isETType(h.Type) && len(data) >= ExtTimestampLen {
			microsecond, _ = DecodeMicrosecond(data)
			msgData = data[ExtTimestampLen:]
		}

		if handler.OnHeader != nil {
			if err := handler.OnHeader(h, microsecond, data); err != nil {
				return err
			}
		}

		if err := dispatch(h, microsecond, msgData, handler); err != nil {
			return err
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
