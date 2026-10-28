package fixture

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

func extra1RadiusAdmin(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("aaa-radius-admin takes no arguments: %q", args)
	}
	if err := extra1TouchReady(); err != nil {
		return err
	}
	mock, address, err := extra1StartRadiusMock(ctx)
	if err != nil {
		return err
	}
	defer extra1StopRadiusMock(mock)
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "mock at %s\n", address)
	config := fmt.Sprintf(`bgp {
	router-id 203.0.113.253;
	peer peer1 {
		connection { remote { ip 127.0.0.1; } local { ip 127.0.0.1; accept false; } }
		session { asn { local 65533; remote 65533; } }
	}
}
system {
	authentication {
		radius {
			server %s { port %s; key "ze-mock-key"; }
			timeout 2;
		}
	}
	authorization {
		profile admin { run { default-action allow; } edit { default-action allow; } }
	}
}
environment { ssh { enabled true; server main { ip 127.0.0.1; port 0; } } }
`, host, port)
	daemon, sshPort, err := extra1RunDaemon(ctx, "aaa-radius-admin.conf", config, nil)
	if err != nil {
		return err
	}
	defer daemon.stop()
	fmt.Fprintf(os.Stderr, "ssh on :%s\n", sshPort)
	configDir, err := extra1InitCLI(ctx, sshPort)
	if err != nil {
		return err
	}
	defer os.RemoveAll(configDir) //nolint:errcheck // fixture cleanup
	if err := extra1Wait(ctx, 500*time.Millisecond); err != nil {
		return err
	}
	if output, err := extra1CLI(ctx, configDir, sshPort, "admin", "testpass", "show bgp"); err != nil {
		return fmt.Errorf("show summary via RADIUS auth: %w\nOUTPUT: %s\n%s", err, output, daemon.contents())
	}
	fmt.Fprintln(os.Stderr, "OK: summary ran via RADIUS auth")
	// RFC 3579 Section 3: the server, not the client, says which port the
	// Access-Request named.
	if !strings.Contains(mock.contents(), "nas-port-id=ssh") {
		return fmt.Errorf("the RADIUS server read no NAS-Port-Id ssh: %s", mock.contents())
	}
	fmt.Fprintln(os.Stderr, "OK: RADIUS server read NAS-Port-Id ssh")
	if err := extra1Wait(ctx, 300*time.Millisecond); err != nil {
		return err
	}
	if !Poll(ctx, 20, 100*time.Millisecond, func() bool {
		return strings.Contains(daemon.contents(), "auth success") && strings.Contains(daemon.contents(), "source=radius")
	}) {
		return fmt.Errorf("daemon did not record RADIUS authentication: %s", daemon.contents())
	}
	fmt.Fprint(os.Stderr, daemon.contents())
	return nil
}

func extra1RadiusFallback(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("aaa-radius-fallback takes no arguments: %q", args)
	}
	if err := extra1TouchReady(); err != nil {
		return err
	}
	config := fmt.Sprintf(`bgp {
	router-id 203.0.113.253;
	peer peer1 {
		connection { remote { ip 127.0.0.1; } local { ip 127.0.0.1; accept false; } }
		session { asn { local 65533; remote 65533; } }
	}
}
system {
	authentication {
		user admin { password %q; profile [ admin ]; }
		radius {
			server 127.0.0.1 { port 1; key "ignored"; }
			timeout 1;
			retries 1;
		}
	}
	authorization {
		profile admin { run { default-action allow; } edit { default-action allow; } }
	}
}
environment { ssh { enabled true; server main { ip 127.0.0.1; port 0; } } }
`, extra1AdminHash)
	daemon, sshPort, err := extra1RunDaemon(ctx, "aaa-radius-fallback.conf", config, nil)
	if err != nil {
		return err
	}
	defer daemon.stop()
	fmt.Fprintf(os.Stderr, "ssh on :%s\n", sshPort)
	if err := extra1Wait(ctx, 500*time.Millisecond); err != nil {
		return err
	}
	if err := extra1RequireCommand(sshPort, "admin", "testpass", "show bgp"); err != nil {
		return fmt.Errorf("show summary via local fallback after RADIUS unreachable: %w\n%s", err, daemon.contents())
	}
	fmt.Fprintln(os.Stderr, "OK: summary ran via local fallback")
	if err := extra1Wait(ctx, 300*time.Millisecond); err != nil {
		return err
	}
	if !Poll(ctx, 20, 100*time.Millisecond, func() bool {
		return strings.Contains(daemon.contents(), "auth success") && strings.Contains(daemon.contents(), "source=local")
	}) {
		return fmt.Errorf("daemon did not record local fallback authentication: %s", daemon.contents())
	}
	if strings.Contains(daemon.contents(), "auth success") && strings.Contains(daemon.contents(), "source=radius") {
		return fmt.Errorf("RADIUS silently succeeded: %s", daemon.contents())
	}
	fmt.Fprint(os.Stderr, daemon.contents())
	return nil
}

func extra1AnswerUnknownCommand(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("answer-unknown-command takes no arguments: %q", args)
	}
	if err := extra1TouchReady(); err != nil {
		return err
	}
	config := fmt.Sprintf(`bgp {
	router-id 203.0.113.253;
	peer peer1 {
		connection { remote { ip 127.0.0.2; } local { ip 127.0.0.1; accept false; } }
		session { asn { local 65533; remote 65533; } }
	}
}
system {
	authentication { user admin { password %q; profile [ admin ]; } }
	authorization { profile admin { run { default-action allow; } edit { default-action allow; } } }
}
environment { ssh { enabled true; server main { ip 127.0.0.1; port 0; } } }
`, extra1AdminHash)
	daemon, sshPort, err := extra1RunDaemon(ctx, "answer-unknown-command.conf", config, nil)
	if err != nil {
		return err
	}
	defer daemon.stop()
	fmt.Fprintf(os.Stderr, "SSH port: %s\n", sshPort)
	if err := extra1Wait(ctx, 500*time.Millisecond); err != nil {
		return err
	}
	output, commandErr := extra1Command(sshPort, "admin", "testpass", "shwo bgp peers")
	if commandErr == nil {
		return fmt.Errorf("an unknown command succeeded: %s", output)
	}
	if output != "" {
		return fmt.Errorf("an unknown command wrote to stdout: %s", output)
	}
	if !strings.Contains(commandErr.Error(), "unknown command") {
		return fmt.Errorf("failure does not name the unknown command: %w", commandErr)
	}
	for _, prefix := range []string{"top ", "row ", "bad ", "end ", "nay "} {
		if strings.HasPrefix(commandErr.Error(), prefix) {
			return fmt.Errorf("a frame line reached the operator: %w", commandErr)
		}
	}
	fmt.Fprintln(os.Stderr, "OK: the typo was reported as an unknown command")
	if err := extra1RequireCommand(sshPort, "admin", "testpass", "show version"); err != nil {
		return fmt.Errorf("a command that exists did not answer: %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK: a command that exists is not reported as unknown")
	return nil
}

// extra1RadiusChap logs an operator in over SSH with `auth-method chap`, so the
// credential is verified by a server that computes the digest itself rather
// than by ze checking its own arithmetic.
//
// The mock rejects an Access-Request carrying both a User-Password and a
// CHAP-Password (RFC 2865 Section 4.1), so a green run also proves the PAP
// credential is absent: a builder that appended instead of selecting would fail
// the login here.
func extra1RadiusChap(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("aaa-radius-chap takes no arguments: %q", args)
	}
	if err := extra1TouchReady(); err != nil {
		return err
	}
	mock, address, err := extra1StartRadiusMock(ctx)
	if err != nil {
		return err
	}
	defer extra1StopRadiusMock(mock)
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "mock at %s\n", address)
	config := fmt.Sprintf(`bgp {
	router-id 203.0.113.253;
	peer peer1 {
		connection { remote { ip 127.0.0.1; } local { ip 127.0.0.1; accept false; } }
		session { asn { local 65533; remote 65533; } }
	}
}
system {
	authentication {
		radius {
			server %s { port %s; key "ze-mock-key"; }
			timeout 2;
			auth-method chap;
		}
	}
	authorization {
		profile admin { run { default-action allow; } edit { default-action allow; } }
	}
}
environment { ssh { enabled true; server main { ip 127.0.0.1; port 0; } } }
`, host, port)
	daemon, sshPort, err := extra1RunDaemon(ctx, "aaa-radius-chap.conf", config, nil)
	if err != nil {
		return err
	}
	defer daemon.stop()
	fmt.Fprintf(os.Stderr, "ssh on :%s\n", sshPort)
	configDir, err := extra1InitCLI(ctx, sshPort)
	if err != nil {
		return err
	}
	defer os.RemoveAll(configDir) //nolint:errcheck // fixture cleanup
	if err := extra1Wait(ctx, 500*time.Millisecond); err != nil {
		return err
	}
	if output, err := extra1CLI(ctx, configDir, sshPort, "admin", "testpass", "show bgp"); err != nil {
		return fmt.Errorf("show bgp via RADIUS CHAP auth: %w\nOUTPUT: %s\nMOCK: %s\n%s",
			err, output, mock.contents(), daemon.contents())
	}
	fmt.Fprintln(os.Stderr, "OK: show bgp ran via RADIUS CHAP auth")
	if err := extra1Wait(ctx, 300*time.Millisecond); err != nil {
		return err
	}
	if !Poll(ctx, 20, 100*time.Millisecond, func() bool {
		return strings.Contains(daemon.contents(), "auth success") && strings.Contains(daemon.contents(), "source=radius")
	}) {
		return fmt.Errorf("daemon did not record RADIUS authentication: %s", daemon.contents())
	}
	if !strings.Contains(mock.contents(), "method=chap") {
		return fmt.Errorf("the server did not read a CHAP credential: %s", mock.contents())
	}
	if strings.Contains(mock.contents(), "method=pap") || strings.Contains(mock.contents(), "method=both") {
		return fmt.Errorf("a PAP credential reached the server under auth-method chap: %s", mock.contents())
	}
	fmt.Fprint(os.Stderr, mock.contents())
	fmt.Fprint(os.Stderr, daemon.contents())
	return nil
}

// extra1RadiusEap logs an operator in over SSH with `auth-method eap-mschapv2`,
// so the login runs a multi-round EAP conversation over RADIUS instead of one
// request and one answer.
//
// The mock verifies the Message-Authenticator of every Access-Request itself,
// with its own HMAC-MD5 (internal/test/mock/radius/eap.go), and discards a
// request whose signature does not verify, which RFC 3579 Section 3.1 requires.
// So a signer that covered the wrong octets ends this fixture in a timeout
// rather than in a login. The mock also refuses a round whose State it did not
// issue, which is what makes the State copy of RFC 2865 Section 5.24 load
// bearing here.
func extra1RadiusEap(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("aaa-radius-eap takes no arguments: %q", args)
	}
	if err := extra1TouchReady(); err != nil {
		return err
	}
	mock, address, err := extra1StartRadiusMock(ctx)
	if err != nil {
		return err
	}
	defer extra1StopRadiusMock(mock)
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "mock at %s\n", address)
	config := fmt.Sprintf(`bgp {
	router-id 203.0.113.253;
	peer peer1 {
		connection { remote { ip 127.0.0.1; } local { ip 127.0.0.1; accept false; } }
		session { asn { local 65533; remote 65533; } }
	}
}
system {
	authentication {
		radius {
			server %s { port %s; key "ze-mock-key"; }
			timeout 2;
			auth-method eap-mschapv2;
		}
	}
	authorization {
		profile admin { run { default-action allow; } edit { default-action allow; } }
	}
}
environment { ssh { enabled true; server main { ip 127.0.0.1; port 0; } } }
`, host, port)
	daemon, sshPort, err := extra1RunDaemon(ctx, "aaa-radius-eap.conf", config, nil)
	if err != nil {
		return err
	}
	defer daemon.stop()
	fmt.Fprintf(os.Stderr, "ssh on :%s\n", sshPort)
	configDir, err := extra1InitCLI(ctx, sshPort)
	if err != nil {
		return err
	}
	defer os.RemoveAll(configDir) //nolint:errcheck // fixture cleanup
	if err := extra1Wait(ctx, 500*time.Millisecond); err != nil {
		return err
	}
	if output, err := extra1CLI(ctx, configDir, sshPort, "admin", "testpass", "show bgp"); err != nil {
		return fmt.Errorf("show bgp via RADIUS EAP auth: %w\nOUTPUT: %s\nMOCK: %s\n%s",
			err, output, mock.contents(), daemon.contents())
	}
	fmt.Fprintln(os.Stderr, "OK: show bgp ran via RADIUS EAP auth")
	if err := extra1Wait(ctx, 300*time.Millisecond); err != nil {
		return err
	}
	if !Poll(ctx, 20, 100*time.Millisecond, func() bool {
		return extra1ContainsBoth(daemon.contents(), "auth success", "source=radius")
	}) {
		return fmt.Errorf("daemon did not record RADIUS authentication: %s", daemon.contents())
	}
	if !strings.Contains(mock.contents(), "method=eap") {
		return fmt.Errorf("the server did not read an EAP credential: %s", mock.contents())
	}
	// The Access-Challenge is what separates this path from PAP and CHAP: the
	// login only completes if ze answered a challenge, carried the State back and
	// signed the next request.
	if !strings.Contains(mock.contents(), "reply=Access-Challenge") {
		return fmt.Errorf("the server never challenged, so no EAP conversation ran: %s", mock.contents())
	}
	if !strings.Contains(mock.contents(), "reply=Access-Accept") {
		return fmt.Errorf("the server never accepted the conversation: %s", mock.contents())
	}
	if strings.Contains(mock.contents(), "discarded") {
		return fmt.Errorf("the server discarded a request from ze: %s", mock.contents())
	}
	if strings.Contains(mock.contents(), "method=pap") || strings.Contains(mock.contents(), "method=chap") {
		return fmt.Errorf("a password credential reached the server under auth-method eap-mschapv2: %s", mock.contents())
	}
	fmt.Fprint(os.Stderr, mock.contents())
	fmt.Fprint(os.Stderr, daemon.contents())
	return nil
}

// extra1RadiusUnsignedConfig is the daemon config of the two unsigned-answer
// fixtures: one RADIUS server row (serverOptions holds its extra leaves) and a
// local admin whose password is the RADIUS one, so the chain's fallback is
// observable.
func extra1RadiusUnsignedConfig(host, port, serverOptions string) string {
	return fmt.Sprintf(`bgp {
	router-id 203.0.113.253;
	peer peer1 {
		connection { remote { ip 127.0.0.1; } local { ip 127.0.0.1; accept false; } }
		session { asn { local 65533; remote 65533; } }
	}
}
system {
	authentication {
		user admin { password %q; profile [ admin ]; }
		radius {
			server %s { port %s; key "ze-mock-key";%s }
			timeout 1;
			retries 1;
		}
	}
	authorization {
		profile admin { run { default-action allow; } edit { default-action allow; } }
	}
}
environment { ssh { enabled true; server main { ip 127.0.0.1; port 0; } } }
`, extra1AdminHash, host, port, serverOptions)
}

// extra1RadiusUnsignedAcceptRefused proves the BlastRADIUS presence check
// through the real login path. The mock answers Access-Accept with a correct
// Response Authenticator and no Message-Authenticator (a legacy server). With
// the server row at its default the answer is discarded, the login falls to
// the local user, and the daemon's warning names the leaf that would accept it.
func extra1RadiusUnsignedAcceptRefused(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("aaa-radius-unsigned-accept-refused takes no arguments: %q", args)
	}
	return extra1RadiusUnsignedLogin(ctx, "aaa-radius-unsigned-accept-refused.conf", "", "local")
}

// extra1RadiusUnsignedAcceptLegacy is the pair of the refused fixture: the
// same unsigned mock, the server row set to require-message-authenticator
// false, and the login succeeds through RADIUS.
func extra1RadiusUnsignedAcceptLegacy(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("aaa-radius-unsigned-accept-legacy takes no arguments: %q", args)
	}
	return extra1RadiusUnsignedLogin(ctx, "aaa-radius-unsigned-accept-legacy.conf", " require-message-authenticator false;", "radius")
}

// extra1RadiusUnsignedLogin logs in over SSH against an unsigned mock and
// requires the authentication to be recorded with source=wantSource and never
// with the other source.
func extra1RadiusUnsignedLogin(ctx context.Context, configName, serverOptions, wantSource string) error {
	if err := extra1TouchReady(); err != nil {
		return err
	}
	mock, address, err := extra1StartRadiusMock(ctx, "--sign-responses=false")
	if err != nil {
		return err
	}
	defer extra1StopRadiusMock(mock)
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	daemon, sshPort, err := extra1RunDaemon(ctx, configName, extra1RadiusUnsignedConfig(host, port, serverOptions), nil)
	if err != nil {
		return err
	}
	defer daemon.stop()
	if err := extra1Wait(ctx, 500*time.Millisecond); err != nil {
		return err
	}
	if err := extra1RequireCommand(sshPort, "admin", "testpass", "show bgp"); err != nil {
		return fmt.Errorf("show bgp after login: %w\n%s", err, daemon.contents())
	}
	// The mock answered: without this the refused case is the unreachable
	// fallback case under another name.
	if !strings.Contains(mock.contents(), "Access-Accept") {
		return fmt.Errorf("the mock never answered Access-Accept: %s", mock.contents())
	}
	fmt.Fprintln(os.Stderr, "OK: the mock answered Access-Accept unsigned")
	otherSource := "radius"
	if wantSource == "radius" {
		otherSource = "local"
	}
	if !Poll(ctx, 20, 100*time.Millisecond, func() bool {
		return strings.Contains(daemon.contents(), "auth success") && strings.Contains(daemon.contents(), "source="+wantSource)
	}) {
		return fmt.Errorf("daemon did not record source=%s authentication: %s", wantSource, daemon.contents())
	}
	if strings.Contains(daemon.contents(), "source="+otherSource) {
		return fmt.Errorf("daemon recorded source=%s authentication: %s", otherSource, daemon.contents())
	}
	fmt.Fprint(os.Stderr, daemon.contents())
	return nil
}
