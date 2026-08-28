package fixture

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // L2TP CHAP and RADIUS require MD5 on the wire
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

func tunnelL2TPSessionDriver(peerTID, firstPeerSID uint16, challengeHex string, sessions int, mode string) Driver {
	return func(ctx context.Context, args []string) error {
		port, err := tunnelArgPort(args, 0)
		if err != nil {
			return err
		}
		challenge, _ := tunnelHex(challengeHex)
		conn, target, err := tunnelL2TPDial(port)
		if err != nil {
			return err
		}
		defer conn.Close()
		hostname := "py-peer"
		if mode == "auth-pool" {
			hostname = "py-e2e"
		}
		reply, _, err := tunnelL2TPExchange(ctx, conn, target, tunnelL2TPSCCRQ(peerTID, hostname, challenge), 20, 250*time.Millisecond)
		if err != nil {
			return errors.New("no SCCRP received")
		}
		avps, err := tunnelL2TPParseAVPs(reply)
		if err != nil || len(avps[9]) != 2 || len(avps[11]) == 0 {
			return errors.New("SCCRP missing Challenge or Assigned Tunnel ID")
		}
		localTID := binary.BigEndian.Uint16(avps[9])
		digest := md5.Sum(append(append([]byte{3}, tunnelL2TPTestSecret()...), avps[11]...))
		body := append(tunnelL2TPAVP(true, 0, tunnelL2TPU16(3)), tunnelL2TPAVP(true, 13, digest[:])...)
		if _, err := conn.WriteToUDP(tunnelL2TPControl(localTID, 0, 1, 1, body), target); err != nil {
			return err
		}
		tunnelL2TPDrain(conn, time.Second)
		ns, nr := uint16(2), uint16(1)
		var lastZeSID uint16
		for index := range sessions {
			peerSID := firstPeerSID + uint16(index)
			callSerial := uint32(index + 1)
			if mode == "auth-pool" {
				callSerial = 99
			} else if mode != "stopccn" {
				callSerial = 42
			}
			icrq := append(tunnelL2TPAVP(true, 0, tunnelL2TPU16(10)), tunnelL2TPAVP(true, 14, tunnelL2TPU16(peerSID))...)
			icrq = append(icrq, tunnelL2TPAVP(true, 15, tunnelL2TPU32(callSerial))...)
			if _, err := conn.WriteToUDP(tunnelL2TPControl(localTID, 0, ns, nr, icrq), target); err != nil {
				return err
			}
			icrp, err := tunnelL2TPReadMessage(conn, 11, 2*time.Second)
			if err != nil || len(icrp[14]) != 2 {
				return fmt.Errorf("no ICRP for session %d", index+1)
			}
			zeSID := binary.BigEndian.Uint16(icrp[14])
			lastZeSID = zeSID
			if mode == "incoming" {
				fmt.Printf("OK: ICRP received, ze SID=%d\n", zeSID)
			}
			tunnelL2TPDrain(conn, 100*time.Millisecond)
			ns++
			nr++
			connectSpeed, framing := uint32(10000000), uint32(2)
			if mode == "stopccn" && index == 1 {
				connectSpeed, framing = 56000, 1
			}
			iccn := append(tunnelL2TPAVP(true, 0, tunnelL2TPU16(12)), tunnelL2TPAVP(true, 24, tunnelL2TPU32(connectSpeed))...)
			iccn = append(iccn, tunnelL2TPAVP(true, 19, tunnelL2TPU32(framing))...)
			if _, err := conn.WriteToUDP(tunnelL2TPControl(localTID, zeSID, ns, nr, iccn), target); err != nil {
				return err
			}
			ns++
			tunnelL2TPDrain(conn, 100*time.Millisecond)
			if mode == "cdn" {
				nr++
				cdn := append(tunnelL2TPAVP(true, 0, tunnelL2TPU16(14)), tunnelL2TPAVP(true, 1, append(tunnelL2TPU16(1), tunnelL2TPU16(0)...))...)
				cdn = append(cdn, tunnelL2TPAVP(true, 14, tunnelL2TPU16(peerSID))...)
				_, err = conn.WriteToUDP(tunnelL2TPControl(localTID, zeSID, ns, nr, cdn), target)
				if err != nil {
					return err
				}
				fmt.Println("OK: CDN sent for established session")
				return nil
			}
		}
		switch mode {
		case "incoming":
			fmt.Println("OK: ICCN sent, session should be established")
		case "auth-pool":
			fmt.Printf("OK: session established with auth+pool config, ze SID=%d\n", lastZeSID)
		case "stopccn":
			stop := append(tunnelL2TPAVP(true, 0, tunnelL2TPU16(4)), tunnelL2TPAVP(true, 9, tunnelL2TPU16(peerTID))...)
			stop = append(stop, tunnelL2TPAVP(true, 1, append(tunnelL2TPU16(1), tunnelL2TPU16(0)...))...)
			if _, err := conn.WriteToUDP(tunnelL2TPControl(localTID, 0, ns, nr, stop), target); err != nil {
				return err
			}
			fmt.Println("OK: StopCCN sent with 2 active sessions")
		}
		return nil
	}
}

func tunnelL2TPDrain(conn *net.UDPConn, timeout time.Duration) {
	buffer := make([]byte, 2048)
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	_, _, _ = conn.ReadFromUDP(buffer)
}

func tunnelL2TPReadMessage(conn *net.UDPConn, messageType uint16, timeout time.Duration) (map[uint16][]byte, error) {
	deadline := time.Now().Add(timeout)
	buffer := make([]byte, 2048)
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(deadline)
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			return nil, err
		}
		avps, err := tunnelL2TPParseAVPs(buffer[:n])
		if err == nil && tunnelL2TPMessageType(avps) == messageType {
			return avps, nil
		}
	}
	return nil, errors.New("control message timeout")
}

func tunnelL2TPOutgoingDriver(peerTID, peerSID uint16, requireSessionIDs bool) Driver {
	return func(ctx context.Context, args []string) error {
		peerPort, err := tunnelArgPort(args, 0)
		if err != nil {
			return err
		}
		restPort, err := tunnelArgPort(args, 1)
		if err != nil {
			return err
		}
		conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: peerPort})
		if err != nil {
			return err
		}
		defer conn.Close()

		type restResult struct {
			body map[string]any
			err  error
		}
		restDone := make(chan restResult, 1)
		go func() {
			body, callErr := tunnelL2TPCallREST(ctx, restPort)
			restDone <- restResult{body: body, err: callErr}
		}()

		buffer := make([]byte, 2048)
		var zeTID uint16
		ourNS, ourNR := uint16(0), uint16(0)
		sawOCRQ := false
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			n, address, readErr := conn.ReadFromUDP(buffer)
			if readErr != nil {
				if sawOCRQ {
					break
				}
				continue
			}
			packet := buffer[:n]
			avps, parseErr := tunnelL2TPParseAVPs(packet)
			if parseErr != nil || len(avps[0]) < 2 {
				continue
			}
			ns := binary.BigEndian.Uint16(packet[8:10])
			fresh := ns == ourNR
			if fresh {
				ourNR++
			}
			switch tunnelL2TPMessageType(avps) {
			case 1:
				if !fresh || len(avps[9]) != 2 {
					continue
				}
				zeTID = binary.BigEndian.Uint16(avps[9])
				mandatory := true
				for _, attribute := range []uint16{0, 2, 3, 4, 7, 9, 10} {
					mandatory = mandatory && avps[attribute] != nil
				}
				if !requireSessionIDs {
					fmt.Printf("OK: SCCRQ received msgtype=1 hostname=%q mandatory_ok=%s challenge=%s tiebreaker=%s\n", avps[7], tunnelBoolText(mandatory), tunnelBoolText(avps[11] != nil), tunnelBoolText(avps[5] != nil))
				}
				response := append(tunnelL2TPAVP(true, 0, tunnelL2TPU16(2)), tunnelL2TPAVP(true, 2, []byte{1, 0})...)
				response = append(response, tunnelL2TPAVP(true, 3, tunnelL2TPU32(3))...)
				peerName := "py-lns"
				if requireSessionIDs {
					peerName = "py-lac"
				}
				response = append(response, tunnelL2TPAVP(true, 7, []byte(peerName))...)
				response = append(response, tunnelL2TPAVP(true, 9, tunnelL2TPU16(peerTID))...)
				response = append(response, tunnelL2TPAVP(true, 10, tunnelL2TPU16(8))...)
				if challenge := avps[11]; challenge != nil {
					digest := md5.Sum(append(append([]byte{2}, []byte(tunnelL2TPSecret)...), challenge...))
					response = append(response, tunnelL2TPAVP(true, 13, digest[:])...)
				}
				_, _ = conn.WriteToUDP(tunnelL2TPControl(zeTID, 0, ourNS, ourNR, response), address)
				ourNS++
			case 7:
				if !fresh || len(avps[14]) != 2 {
					continue
				}
				zeSID := binary.BigEndian.Uint16(avps[14])
				sawOCRQ = true
				called := string(avps[21])
				mandatory := true
				for _, attribute := range []uint16{0, 14, 15, 16, 17, 18, 19, 21} {
					mandatory = mandatory && avps[attribute] != nil
				}
				if requireSessionIDs {
					fmt.Printf("OK: OCRQ received msgtype=7 called='%s' mandatory_ok=%s ze_sid=%d\n", called, tunnelBoolText(mandatory), zeSID)
				} else {
					fmt.Printf("OK: OCRQ received msgtype=7 called=b'%s' ze_sid=%d\n", called, zeSID)
				}
				ocrp := append(tunnelL2TPAVP(true, 0, tunnelL2TPU16(8)), tunnelL2TPAVP(true, 14, tunnelL2TPU16(peerSID))...)
				_, _ = conn.WriteToUDP(tunnelL2TPControl(zeTID, zeSID, ourNS, ourNR, ocrp), address)
				ourNS++
				occn := append(tunnelL2TPAVP(true, 0, tunnelL2TPU16(9)), tunnelL2TPAVP(true, 24, tunnelL2TPU32(64000))...)
				occn = append(occn, tunnelL2TPAVP(true, 19, tunnelL2TPU32(1))...)
				_, _ = conn.WriteToUDP(tunnelL2TPControl(zeTID, zeSID, ourNS, ourNR, occn), address)
				ourNS++
			}
		}
		select {
		case result := <-restDone:
			if result.err != nil {
				return fmt.Errorf("outgoing-call RPC errored: %w", result.err)
			}
			if result.body["status"] != "done" {
				return fmt.Errorf("outgoing-call RPC did not complete: %#v", result.body)
			}
			encoded, _ := json.Marshal(result.body)
			if requireSessionIDs {
				fmt.Fprintf(os.Stderr, "RPC-BODY: %s\n", encoded)
			}
			if requireSessionIDs && (!bytes.Contains(encoded, []byte("local-sid")) || !bytes.Contains(encoded, []byte("remote-sid"))) {
				return fmt.Errorf("RPC result missing session identifiers: %s", encoded)
			}
			if requireSessionIDs {
				fmt.Println("OK: outgoing-call established; RPC returned session identifiers")
			} else {
				fmt.Println("OK: outgoing-call RPC returned status=done (tunnel initiated + call up)")
			}
			return nil
		case <-time.After(25 * time.Second):
			return errors.New("outgoing-call RPC did not return")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func tunnelL2TPCallREST(ctx context.Context, port int) (map[string]any, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	base := fmt.Sprintf("http://127.0.0.1:%d/api/v1", port)
	ready := Poll(ctx, 40, 300*time.Millisecond, func() bool {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/commands", nil)
		req.Header.Set("Authorization", "Bearer secret")
		response, err := client.Do(req)
		if err != nil {
			return false
		}
		_ = response.Body.Close()
		return true
	})
	if !ready {
		return nil, errors.New("REST API did not become ready")
	}
	payload := strings.NewReader(`{"command":"request l2tp outgoing-call remote lns1 called 5551234"}`)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, base+"/execute", payload)
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	client.Timeout = 25 * time.Second
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("HTTP %d: %s", response.StatusCode, body)
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return nil, err
	}
	return body, nil
}

func tunnelBoolText(value bool) string {
	if value {
		return "True"
	}
	return "False"
}

func tunnelL2TPEmittedShape(ctx context.Context, args []string) error {
	port, err := tunnelArgPort(args, 0)
	if err != nil {
		return err
	}
	challenge, _ := tunnelHex("00112233445566778899aabbccddeeff")
	conn, target, err := tunnelL2TPDial(port)
	if err != nil {
		return err
	}
	defer conn.Close()
	reply, _, err := tunnelL2TPExchange(ctx, conn, target, tunnelL2TPSCCRQ(0x0301, "py-shape", challenge), 40, 250*time.Millisecond)
	if err != nil {
		return errors.New("no SCCRP received; cannot inspect wire shape")
	}
	failures := make([]string, 0)
	check := func(ok bool, message string) {
		if ok {
			fmt.Println("PASS:", message)
		} else {
			failures = append(failures, message)
			fmt.Fprintln(os.Stderr, "FAIL:", message)
		}
	}
	word := binary.BigEndian.Uint16(reply[:2])
	check(word&0x00f0 == 0, "SCCRP header reserved bits 8-11 zero")
	length := int(binary.BigEndian.Uint16(reply[2:4]))
	offset, firstType := 12, uint16(0xffff)
	reservedOK := true
	for offset+6 <= length {
		avpWord := binary.BigEndian.Uint16(reply[offset : offset+2])
		attribute := binary.BigEndian.Uint16(reply[offset+4 : offset+6])
		if offset == 12 {
			firstType = attribute
		}
		reservedOK = reservedOK && avpWord&0x3c00 == 0
		step := int(avpWord & 0x03ff)
		if step < 6 || offset+step > length {
			break
		}
		offset += step
	}
	check(firstType == 0, "Message Type AVP is first in SCCRP")
	check(reservedOK, "all SCCRP AVP reserved bits 2-5 clear")
	avps, parseErr := tunnelL2TPParseAVPs(reply)
	if parseErr != nil {
		return parseErr
	}
	check(tunnelL2TPMessageType(avps) == 2, "first AVP names SCCRP")
	want := md5.Sum(append(append([]byte{2}, []byte(tunnelL2TPSecret)...), challenge...))
	check(bytes.Equal(avps[13], want[:]), "SCCRP Challenge Response equals independently computed digest")
	if len(avps[9]) != 2 || len(avps[11]) == 0 {
		return errors.New("SCCRP lacked Assigned Tunnel ID or Challenge")
	}
	digest := md5.Sum(append(append([]byte{3}, []byte(tunnelL2TPSecret)...), avps[11]...))
	body := append(tunnelL2TPAVP(true, 0, tunnelL2TPU16(3)), tunnelL2TPAVP(true, 13, digest[:])...)
	_, err = conn.WriteToUDP(tunnelL2TPControl(binary.BigEndian.Uint16(avps[9]), 0, 1, 1, body), target)
	if err != nil {
		return err
	}
	fmt.Println("OK: SCCCN sent with independently computed Challenge Response")
	if len(failures) != 0 {
		return fmt.Errorf("%d wire-shape checks failed", len(failures))
	}
	fmt.Println("OK: all RFC 2661 emitted-shape checks passed")
	return nil
}

func tunnelL2TPZeroTunnelID(ctx context.Context, args []string) error {
	port, err := tunnelArgPort(args, 0)
	if err != nil {
		return err
	}
	conn, target, err := tunnelL2TPDial(port)
	if err != nil {
		return err
	}
	defer conn.Close()
	bad, _, err := tunnelL2TPExchange(ctx, conn, target, tunnelL2TPSCCRQ(0, "py-zero-tid", nil), 40, 250*time.Millisecond)
	if err != nil {
		return errors.New("no reply to zero Assigned Tunnel ID SCCRQ")
	}
	if len(bad) < 12 {
		return errors.New("short StopCCN")
	}
	failures := make([]string, 0)
	check := func(ok bool, message string) {
		if !ok {
			failures = append(failures, message)
			fmt.Fprintln(os.Stderr, "FAIL:", message)
		} else {
			fmt.Println("PASS:", message)
		}
	}
	check(int(binary.BigEndian.Uint16(bad[2:4])) == len(bad), "StopCCN Length field matches datagram")
	check(binary.BigEndian.Uint16(bad[4:6]) == 0, "StopCCN header Tunnel ID is 0")
	check(binary.BigEndian.Uint16(bad[6:8]) == 0, "StopCCN header Session ID is 0")
	check(binary.BigEndian.Uint16(bad[8:10]) == 0, "StopCCN Ns is 0")
	check(binary.BigEndian.Uint16(bad[10:12]) == 1, "StopCCN Nr acknowledges SCCRQ")
	check(len(bad) >= 20 && binary.BigEndian.Uint16(bad[16:18]) == 0, "Message Type AVP is first")
	avps, err := tunnelL2TPParseAVPs(bad)
	if err != nil {
		return err
	}
	check(tunnelL2TPMessageType(avps) == 4, "reply is StopCCN")
	check(len(avps[9]) == 2 && binary.BigEndian.Uint16(avps[9]) != 0, "StopCCN Assigned Tunnel ID is non-zero")
	check(len(avps[1]) >= 4, "Result Code carries Error Code")
	if len(avps[1]) >= 4 {
		check(binary.BigEndian.Uint16(avps[1][0:2]) == 2, "Result Code is 2")
		check(binary.BigEndian.Uint16(avps[1][2:4]) == 3, "Error Code is 3")
	}
	good, _, err := tunnelL2TPExchange(ctx, conn, target, tunnelL2TPSCCRQ(0x0303, "py-good-tid", nil), 40, 250*time.Millisecond)
	if err != nil {
		return errors.New("no reply to well-formed SCCRQ")
	}
	goodAVPs, err := tunnelL2TPParseAVPs(good)
	if err != nil {
		return err
	}
	check(binary.BigEndian.Uint16(good[4:6]) == 0x0303, "SCCRP addressed with peer Assigned Tunnel ID")
	check(len(good) >= 20 && binary.BigEndian.Uint16(good[16:18]) == 0, "SCCRP Message Type AVP is first")
	check(tunnelL2TPMessageType(goodAVPs) == 2, "non-zero Assigned Tunnel ID gets SCCRP")
	if len(failures) != 0 {
		return fmt.Errorf("%d zero-tunnel-id checks failed", len(failures))
	}
	fmt.Println("OK: all RFC 2661 zero Assigned Tunnel ID checks passed")
	return nil
}
