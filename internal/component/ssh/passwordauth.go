// Design: plan/spec-fixit-bcrypt-hash-credential.md -- hash-as-token is local-only
// Related: ssh.go -- WithPasswordAuth callback delegates here

package ssh

import (
	"net"

	"codeberg.org/thomas-mangin/ze/internal/component/authz"
	"codeberg.org/thomas-mangin/ze/internal/core/redact"
)

// loggedCommand sanitizes an SSH exec command for the operational log. It
// redacts credential tokens (bcrypt hashes and password-key values) FIRST, then
// truncates, so a secret straddling the truncation boundary can never
// half-leak. A one-shot `ze config set ... password <hash>` would otherwise be
// written verbatim at Info. The full, unredacted command still flows to the
// executor; only this logged form is scrubbed.
func loggedCommand(cmd string) string {
	return truncateForLog(redact.Command(cmd))
}

// authenticatePassword runs the SSH password-auth decision for one connection.
// It is the single producer of AuthRequest.Local: the accepted socket peer is
// classified by isLocalTransport, so the bcrypt-hash-as-token credential path
// is enabled only for a loopback/unix peer (the on-box ze CLI) and rejected for
// every remote peer. Extracted from the wish callback so the transport gate is
// unit-testable without standing up a live SSH server.
func (s *Server) authenticatePassword(authenticator authz.Authenticator, username, pass string, peer net.Addr) bool {
	remote := ""
	if peer != nil {
		remote = peer.String()
	}
	result, err := authenticator.Authenticate(authz.AuthRequest{
		Username:   username,
		Password:   pass,
		RemoteAddr: remote,
		Service:    "ssh",
		Local:      isLocalTransport(peer),
	})
	if err == nil && result.Authenticated {
		s.logger.Info("SSH auth success",
			"username", username, "remote", remote,
			"source", result.Source,
			"profiles", truncateProfiles(result.Profiles))
		return true
	}
	s.logger.Warn("SSH auth failure", "username", username, "remote", remote)
	s.recordAuthFailure(username, remote)
	return false
}

// isLocalTransport reports whether an accepted connection originated from a
// trusted-local transport: a unix-socket peer or a loopback TCP peer. It derives
// its answer solely from the accepted socket address (ctx.RemoteAddr(), set by
// the server from the OS-level peer), never from client-supplied data, so a
// remote client cannot spoof a local classification. See A-4 in the spec:
// charmbracelet/ssh denies port forwarding by default, so a remote peer cannot
// reach ze's listener via a forged loopback source through ze's own server.
//
// Fail-closed: a nil address, or any address not positively recognized as unix
// or loopback TCP, classifies as remote (false).
func isLocalTransport(addr net.Addr) bool {
	if addr == nil {
		return false
	}
	switch a := addr.(type) {
	case *net.UnixAddr:
		return true
	case *net.TCPAddr:
		return a.IP.IsLoopback()
	}
	// Unknown concrete type: still fail closed. A "unix" network is local;
	// otherwise the host portion must parse to a loopback IP.
	if addr.Network() == "unix" {
		return true
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
