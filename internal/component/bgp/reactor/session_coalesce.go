// Design: docs/architecture/core-design.md — IPv4 unicast UPDATE coalescing
// Related: session_read.go — standard (non-coalescing) read path

package reactor

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wire"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

var _ = env.MustRegister(env.EnvEntry{
	Key:         "ze.bgp.reactor.coalesce",
	Type:        "bool",
	Default:     "true",
	Description: "Coalesce consecutive IPv4 unicast UPDATEs sharing identical attributes",
})

// coalesceState holds a pending UPDATE whose NLRIs may be extended
// by subsequent UPDATEs carrying identical path attributes.
// Embedded in Session to avoid per-batch heap allocation.
//
// The body slice uses len() as write position and cap() as space limit.
// It is carved from the pool buffer: buf.Buf[:written] with cap(buf.Buf).
//
// Original per-message pool buffers are returned immediately after their
// NLRI bytes are copied into body. Only the coalesce pool buffer is held.
type coalesceState struct {
	buf     BufHandle // pool buffer backing body
	body    []byte    // synthetic UPDATE body; len=written, cap=available
	attrLen int       // cached attribute length for fast pre-check
}

// coalesceEnabled returns true if UPDATE coalescing is active.
func coalesceEnabled() bool {
	return env.GetBool("ze.bgp.reactor.coalesce", true)
}

// readAndProcessCoalesced reads one BGP message and either processes it
// immediately or accumulates it into the held coalesce batch. After each
// read, checks bufReader.Buffered() to decide whether to flush.
func (s *Session) readAndProcessCoalesced(conn net.Conn, bufReader *bufio.Reader) error {
	buf := s.getReadBuffer()
	if buf.Buf == nil {
		if err := s.flushCoalesce(); err != nil {
			return err
		}
		return fmt.Errorf("read buffer exhausted: pool at maximum allocation")
	}

	kept := false
	defer func() {
		if !kept {
			s.returnReadBuffer(buf)
		}
	}()

	_, err := io.ReadFull(bufReader, buf.Buf[:message.HeaderLen])
	if err != nil {
		s.resetCoalesce()
		if errors.Is(err, io.EOF) || isConnectionReset(err) {
			s.handleConnectionClose()
			return ErrConnectionClosed
		}
		if s.prefixMetrics != nil {
			s.prefixMetrics.wireReadErrors.With(s.settings.Address.String()).Inc()
		}
		return err
	}

	s.recentRead.Store(true)

	hdr, err := message.ParseHeader(buf.Buf[:message.HeaderLen])
	if err != nil {
		s.resetCoalesce()
		s.logFSMEvent(fsm.EventBGPHeaderErr)
		return fmt.Errorf("parse header: %w", err)
	}

	if err := hdr.ValidateLengthWithMax(s.extendedMessage); err != nil {
		s.resetCoalesce()
		var lengthBuf [2]byte
		binary.BigEndian.PutUint16(lengthBuf[:], hdr.Length)
		s.logNotifyErr(conn,
			message.NotifyMessageHeader,
			message.NotifyHeaderBadLength,
			lengthBuf[:],
		)
		s.logFSMEvent(fsm.EventBGPHeaderErr)
		s.closeConn()
		return fmt.Errorf("message length %d exceeds max for %s: %w", hdr.Length, hdr.Type, err)
	}

	bodyLen := int(hdr.Length) - message.HeaderLen
	if bodyLen > 0 {
		_, err = io.ReadFull(bufReader, buf.Buf[message.HeaderLen:hdr.Length])
		if err != nil {
			s.resetCoalesce()
			return fmt.Errorf("read body: %w", err)
		}
	}

	// Counts actual wire bytes, not coalesced synthetic size.
	if s.prefixMetrics != nil {
		s.prefixMetrics.wireBytesRecv.With(s.settings.Address.String()).Add(float64(hdr.Length))
	}

	body := buf.Buf[message.HeaderLen:hdr.Length]

	if hdr.Type != message.TypeUPDATE {
		if err := s.flushCoalesce(); err != nil {
			return err
		}
		var processErr error
		processErr, kept = s.processMessage(&hdr, body, buf)
		return processErr
	}

	sections, parseErr := wire.ParseUpdateSections(body)
	if parseErr != nil {
		if err := s.flushCoalesce(); err != nil {
			return err
		}
		var processErr error
		processErr, kept = s.processMessage(&hdr, body, buf)
		return processErr
	}

	wdLen := sections.WithdrawnLen()
	nlriBytes := sections.NLRI(body)
	attrBytes := sections.Attrs(body)
	attrLen := sections.AttrsLen()

	if wdLen != 0 || len(nlriBytes) == 0 {
		if err := s.flushCoalesce(); err != nil {
			return err
		}
		var processErr error
		processErr, kept = s.processMessage(&hdr, body, buf)
		return processErr
	}

	coal := &s.coalesce
	if coal.body != nil {
		if coal.attrLen == attrLen &&
			cap(coal.body)-len(coal.body) >= len(nlriBytes) &&
			bytes.Equal(coal.body[4:4+coal.attrLen], attrBytes) {
			coal.body = append(coal.body, nlriBytes...)
			if bufReader.Buffered() == 0 {
				return s.flushCoalesce()
			}
			return nil
		}
		if err := s.flushCoalesce(); err != nil {
			return err
		}
	}

	coalBuf := s.getReadBuffer()
	if coalBuf.Buf == nil {
		var processErr error
		processErr, kept = s.processMessage(&hdr, body, buf)
		return processErr
	}

	off := 0
	coalBuf.Buf[off] = 0
	coalBuf.Buf[off+1] = 0
	off += 2

	binary.BigEndian.PutUint16(coalBuf.Buf[off:], uint16(attrLen))
	off += 2

	copy(coalBuf.Buf[off:], attrBytes)
	off += attrLen

	copy(coalBuf.Buf[off:], nlriBytes)
	off += len(nlriBytes)

	maxBody := message.MaxMsgLen - message.HeaderLen
	if s.extendedMessage {
		maxBody = message.ExtMsgLen - message.HeaderLen
	}
	bodyCap := min(len(coalBuf.Buf), maxBody)

	coal.buf = coalBuf
	coal.body = coalBuf.Buf[:off:bodyCap]
	coal.attrLen = attrLen

	if bufReader.Buffered() == 0 {
		return s.flushCoalesce()
	}
	return nil
}

// flushCoalesce dispatches the held coalesced UPDATE through processMessage,
// then releases the coalesce pool buffer. Returns nil if nothing held.
func (s *Session) flushCoalesce() error {
	coal := &s.coalesce
	if coal.body == nil {
		return nil
	}

	coalBuf := coal.buf
	body := coal.body

	coal.buf = BufHandle{}
	coal.body = nil
	coal.attrLen = 0

	totalLen := message.HeaderLen + len(body)
	hdr := message.Header{
		Length: uint16(totalLen),
		Type:   message.TypeUPDATE,
	}

	processErr, kept := s.processMessage(&hdr, body, coalBuf)
	if !kept {
		s.returnReadBuffer(coalBuf)
	}
	return processErr
}

// resetCoalesce releases any held coalesce state without dispatching.
func (s *Session) resetCoalesce() {
	coal := &s.coalesce
	if coal.body == nil {
		return
	}
	s.returnReadBuffer(coal.buf)
	coal.buf = BufHandle{}
	coal.body = nil
	coal.attrLen = 0
}
