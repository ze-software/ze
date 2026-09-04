// Design: docs/architecture/testing/interop.md -- what a checker owes, and the five vacuity traps.
// Overview: radius.go -- the suite, its peers, and the scenario registry these checkers serve.
// Related: docs/guide/radius.md -- the chain order, the profile mapping and the CHAP tradeoff proven here.
//
// Every scenario reads BOTH sides. Ze's log saying source=radius is not enough
// on its own: a login the local bcrypt backend satisfied produces a log line of
// the same shape and no server traffic at all. So each assertion that a RADIUS
// login happened is paired with the server's own record of the request it
// answered, and that record carries the NAS-Identifier ze sent, so it names ze
// rather than anything else on the lab network.
package radius

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	// radiusUser exists in BOTH the server's user file and ze's local account
	// list, with different passwords. That overlap is what lets AC-5 prove the
	// chain stops at an Access-Reject instead of falling through to local.
	radiusUser = "radiusop"
	// localUser exists only locally, and the lab server answers nothing for it.
	// A login as localUser that reports source=local is the positive control:
	// it proves the SSH listener and the local bcrypt backend are both live, so
	// a refused radiusUser login cannot be explained by a broken daemon.
	localUser = "localop"

	// localPassword is the bcrypt fixture password both local accounts carry.
	// The hash is published in the scenario ze.conf files.
	localPassword = "testpass"
	// papPassword and chapPassword are the passwords the server holds for
	// radiusUser in the PAP and the two CHAP scenarios. Both are published
	// fixture values that protect nothing, and the scenario user files carry
	// the matching entries in this same repository.
	papPassword  = "fixture-pap-secret"  //nolint:gosec // G101: a published lab fixture, matched by test/interop-radius/scenarios/*/users.
	chapPassword = "fixture-chap-secret" //nolint:gosec // G101: a published lab fixture, matched by test/interop-radius/scenarios/*/users.
	eapPassword  = "fixture-eap-secret"  //nolint:gosec // G101: a published lab fixture, matched by test/interop-radius/scenarios/*/users.

	// deniedCommand is denied by the interop-operator profile and allowed by
	// admin. allowedCommand is allowed by both.
	deniedCommand  = "show bgp"
	allowedCommand = "show version"

	// unauthorizedMessage is plugin.UnauthorizedMessage, the text an operator
	// reads when authorization refuses a command. Only authorization produces
	// it, so asserting it distinguishes a refused command from an unknown one.
	unauthorizedMessage = "command restricted by access control"

	sourceRADIUS = "radius"
	sourceLocal  = "local"

	verdictAccept = "accept"
	verdictReject = "reject"
	// verdictStateEcho is one intermediate round of an EAP conversation: a
	// request that arrived carrying both an EAP-Message and the State the server
	// issued. It is the only evidence that the login was a conversation at all,
	// because an accept looks the same whether it took one request or four, and
	// FreeRADIUS runs no post-auth section for the Access-Challenge that asked
	// the question.
	verdictStateEcho = "state-echo"
	// verdictSilent is the lab server's deliberate non-answer for localUser.
	// It is recorded rather than inferred, so the control proves the server SAW
	// the request and chose silence, instead of proving nothing arrived.
	verdictSilent = "silent"

	credentialPresent = "present"
	credentialAbsent  = "absent"

	// probeNAS names the radclient control probe in the server's record, so its
	// line can never be read as a request ze sent.
	probeNAS = "radclient-probe"

	cliStore      = "/var/lib/ze-cli-store"
	cliExitMarker = "ZE-CLI-EXIT="
	cliInitFailed = "ZE-CLI-INIT-FAILED"

	zeLogLines = 2000
)

// scenarioLab is the typed handle each checker drives. It holds no Docker
// state of its own: every observation goes through the protocol-neutral
// CheckerLab, so a leaf test can supply one without starting a container.
type scenarioLab struct {
	lab     interoplab.CheckerLab
	timeout time.Duration
}

// loginResult is one measured `ze cli` run. ExitCode is read from a marker the
// shell prints, never from the Docker exec status, so a Docker failure and a
// refused login can never be confused.
type loginResult struct {
	Output   string
	ExitCode int
}

// serverRecord is one line the lab's linelog module wrote: what FreeRADIUS
// received and what it answered. The credential fields carry PRESENCE only,
// because the fixture must not put a password or a digest in a log file.
type serverRecord struct {
	Verdict       string
	User          string
	UserPassword  string
	CHAPPassword  string
	EAPMessage    string
	NASIdentifier string
}

// recordWant is a fully specified expectation. Every field is compared, so a
// checker cannot accidentally assert less than it means to.
type recordWant struct {
	Verdict       string
	User          string
	UserPassword  string
	CHAPPassword  string
	EAPMessage    string
	NASIdentifier string
}

func (w recordWant) matches(record serverRecord) bool {
	return w.Verdict == record.Verdict &&
		w.User == record.User &&
		w.UserPassword == record.UserPassword &&
		w.CHAPPassword == record.CHAPPassword &&
		w.EAPMessage == record.EAPMessage &&
		w.NASIdentifier == record.NASIdentifier
}

// credentialShape says which credential the scenario's auth-method puts on the
// wire, and therefore which pair of fields the server records. It is passed in
// rather than inferred, because the whole point of reading the server's record
// is that it says what arrived rather than what ze meant to send.
type credentialShape struct {
	UserPassword string
	CHAPPassword string
	EAPMessage   string
}

var (
	papShape  = credentialShape{UserPassword: credentialPresent, CHAPPassword: credentialAbsent, EAPMessage: credentialAbsent}
	chapShape = credentialShape{UserPassword: credentialAbsent, CHAPPassword: credentialPresent, EAPMessage: credentialAbsent}
	// eapShape carries NEITHER password attribute: RFC 3579 Section 2.1 puts the
	// credential inside the EAP conversation, so a User-Password or a
	// CHAP-Password beside an EAP-Message is ze sending a second credential the
	// operator did not configure.
	eapShape = credentialShape{UserPassword: credentialAbsent, CHAPPassword: credentialAbsent, EAPMessage: credentialPresent}
)

// wantRecord builds the fully specified expectation for one request ze sent.
func wantRecord(verdict, user string, shape credentialShape) recordWant {
	return recordWant{
		Verdict:       verdict,
		User:          user,
		UserPassword:  shape.UserPassword,
		CHAPPassword:  shape.CHAPPassword,
		EAPMessage:    shape.EAPMessage,
		NASIdentifier: nasIdentifier,
	}
}

func (w recordWant) String() string {
	var tb textbuf.Buffer
	return tb.Str("verdict=").Str(w.Verdict).Str(" user=").Str(w.User).
		Str(" user-password=").Str(w.UserPassword).
		Str(" chap-password=").Str(w.CHAPPassword).
		Str(" eap-message=").Str(w.EAPMessage).
		Str(" nas-identifier=").Str(w.NASIdentifier).String()
}

// checkPAP proves the default credential works against a server ze did not
// write, that the reply's Filter-Id decides what the operator may run, and that
// an Access-Reject stops the chain rather than handing the login to local.
func checkPAP(ctx context.Context, lab *scenarioLab) error {
	return checkAcceptedLogin(ctx, lab, papShape, papPassword, "RADIUS")
}

// checkCHAP proves a server ze did not write reproduces the digest ze computed.
// The server holds the password in cleartext, which RFC 2865 Section 2.2
// requires, and the record shows a CHAP-Password and no User-Password, which is
// what Section 4.1 demands of an Access-Request.
func checkCHAP(ctx context.Context, lab *scenarioLab) error {
	return checkAcceptedLogin(ctx, lab, chapShape, chapPassword, "RADIUS CHAP")
}

// checkEAP proves ze completes a multi-round EAP conversation with a server ze
// did not write.
//
// It is the only scenario whose evidence includes an intermediate round. The
// server's record of an Access-Challenge is what says the login was a
// conversation: RFC 3579 Section 3.1 puts the EAP packets in EAP-Message
// attributes and RFC 2865 Section 5.24 makes ze copy the State back, and
// neither obligation exists for a login that took one request.
//
// It also proves what a mock cannot. FreeRADIUS computes the Message-
// Authenticator, the MS-CHAPv2 NT-Response and the authenticator response from
// its own code, so ze's arithmetic has to agree with an implementation ze does
// not share a line with.
func checkEAP(ctx context.Context, lab *scenarioLab) error {
	if err := lab.assertLocalControl(ctx, eapShape); err != nil {
		return err
	}

	result, err := lab.login(ctx, radiusUser, eapPassword, allowedCommand)
	if err != nil {
		return fmt.Errorf("assertion 2: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("assertion 2: %s could not run %q through RADIUS EAP-MSCHAPv2 (exit %d): %s",
			radiusUser, allowedCommand, result.ExitCode, result.Output)
	}
	if err := lab.assertAuthSource(ctx, radiusUser, sourceRADIUS); err != nil {
		return fmt.Errorf("assertion 2: %w", err)
	}

	// The conversation, not just its verdict. RFC 2865 Section 5.24 makes ze
	// return the State "unmodified", and this record is the server's own
	// evidence that it arrived: a login that reached Access-Accept with no round
	// carrying State before it did not run EAP at all.
	if err := lab.assertServerRecord(ctx, wantRecord(verdictStateEcho, radiusUser, eapShape)); err != nil {
		return fmt.Errorf("assertion 3: %w", err)
	}
	if err := lab.assertServerRecord(ctx, wantRecord(verdictAccept, radiusUser, eapShape)); err != nil {
		return fmt.Errorf("assertion 4: %w", err)
	}

	if err := lab.assertFilterIDProfile(ctx, eapPassword); err != nil {
		return fmt.Errorf("assertion 5: %w", err)
	}

	return lab.assertRejectStopsChain(ctx, localPassword, eapShape, 6)
}

// checkAcceptedLogin is the walk both accepting scenarios take. Only three
// things separate PAP from CHAP here: which credential shape the server records,
// which password the user file holds, and what the failure message calls the
// method. Everything else is one sequence, so it is written once. A second copy
// would be a future disagreement about what an accepted login owes, with
// nothing to arbitrate it (ai/rules/principles.md).
//
// The scenario that has its OWN shape is checkCHAPHashed, and it stays separate
// because it asserts a REFUSAL: sharing this body would mean a flag deciding
// whether each assertion is expected to pass or fail.
func checkAcceptedLogin(ctx context.Context, lab *scenarioLab, shape credentialShape, password, method string) error {
	if err := lab.assertLocalControl(ctx, shape); err != nil {
		return err
	}

	// AC-2 and AC-3: the login itself, over ze's real SSH listener.
	result, err := lab.login(ctx, radiusUser, password, allowedCommand)
	if err != nil {
		return fmt.Errorf("assertion 2: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("assertion 2: %s could not run %q through %s (exit %d): %s",
			radiusUser, allowedCommand, method, result.ExitCode, result.Output)
	}
	if err := lab.assertAuthSource(ctx, radiusUser, sourceRADIUS); err != nil {
		return fmt.Errorf("assertion 2: %w", err)
	}

	// AC-4: the server's own record of that request, and of which credential
	// arrived. Without it, a login the local backend satisfied would look
	// identical in ze's log.
	if err := lab.assertServerRecord(ctx, wantRecord(verdictAccept, radiusUser, shape)); err != nil {
		return fmt.Errorf("assertion 3: %w", err)
	}

	// AC-6: the profile the Access-Accept named is the one that governs.
	if err := lab.assertFilterIDProfile(ctx, password); err != nil {
		return fmt.Errorf("assertion 4: %w", err)
	}

	// AC-5: the wrong password is the RIGHT password for the local account of
	// the same name, so a chain that fell through would let this login succeed.
	return lab.assertRejectStopsChain(ctx, localPassword, shape, 5)
}

// checkCHAPHashed proves the consequence RFC 2865 Section 2.2 states and
// docs/guide/radius.md documents as the operator's cost for auth-method chap:
// "If the password is not available in cleartext to the RADIUS server then the
// server MUST send an Access-Reject to the client."
//
// A rejection on its own would also follow from a typo in the user file, so the
// scenario first proves with radclient that the SAME entry accepts the SAME
// password over PAP. The storage form is then the only thing left to explain
// the CHAP rejection.
func checkCHAPHashed(ctx context.Context, lab *scenarioLab) error {
	if err := lab.assertLocalControl(ctx, chapShape); err != nil {
		return err
	}

	// AC-7, first half: the fixture entry is good.
	if err := lab.assertPAPProbeAccepted(ctx, radiusUser, chapPassword); err != nil {
		return fmt.Errorf("assertion 2: %w", err)
	}

	// AC-7, second half: the same entry refuses CHAP.
	result, err := lab.login(ctx, radiusUser, chapPassword, allowedCommand)
	if err != nil {
		return fmt.Errorf("assertion 3: %w", err)
	}
	if result.ExitCode == 0 {
		return fmt.Errorf("assertion 3: %w", lab.explainUnexpectedLogin(ctx, radiusUser, result.Output))
	}

	if err := lab.assertServerRecord(ctx, wantRecord(verdictReject, radiusUser, chapShape)); err != nil {
		return fmt.Errorf("assertion 4: %w", err)
	}

	if err := lab.assertNoAuthSuccess(ctx, radiusUser); err != nil {
		return fmt.Errorf("assertion 5: %w", err)
	}
	return lab.assertAuthFailureRecorded(ctx, radiusUser, 6)
}

// explainUnexpectedLogin names the side at fault when the CHAP login the
// hashed-password scenario expects to be refused succeeds instead.
//
// Two different defects produce that one exit status: FreeRADIUS accepted a
// digest it cannot verify from a hash, or ze answered the login from a local
// account after the Access-Reject. The exit status alone cannot tell them
// apart, so the source ze recorded decides. Reporting the first when it was
// the second sends the reader to FreeRADIUS to look for a defect in ze's chain.
func (l *scenarioLab) explainUnexpectedLogin(ctx context.Context, username, output string) error {
	text, err := l.readZeLog(ctx)
	if err != nil {
		return err
	}
	if slices.Contains(authSuccessSources(text, username), sourceLocal) {
		return fmt.Errorf("ze authenticated %s through the local backend while the server rejected the request; the chain did not stop: %s",
			username, output)
	}
	return fmt.Errorf("CHAP login as %s succeeded against a server holding a hashed password; "+
		"RFC 2865 Section 2.2 requires an Access-Reject: %s", username, output)
}

// assertLocalControl runs the positive control every scenario opens with.
//
// The chain reaches the local backend only when RADIUS gives no answer at all,
// because an Access-Reject stops it (docs/guide/radius.md, steps 5 and 7). The
// lab server therefore holds no entry for localUser and stays silent, and it
// records the silence, so this control proves three things at once: the server
// saw the request, ze timed out and fell through, and the SSH listener with the
// local bcrypt backend behind it accepts a correct credential.
func (l *scenarioLab) assertLocalControl(ctx context.Context, shape credentialShape) error {
	result, err := l.login(ctx, localUser, localPassword, allowedCommand)
	if err != nil {
		return fmt.Errorf("assertion 1: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("assertion 1: local control login as %s failed (exit %d): %s",
			localUser, result.ExitCode, result.Output)
	}
	if err := l.assertAuthSource(ctx, localUser, sourceLocal); err != nil {
		return fmt.Errorf("assertion 1: %w", err)
	}
	if err := l.assertServerRecord(ctx, wantRecord(verdictSilent, localUser, shape)); err != nil {
		return fmt.Errorf("assertion 1: %w", err)
	}
	return nil
}

// assertFilterIDProfile proves the profile named in the Access-Accept Filter-Id
// is the profile that governs the session, by observing the authorization
// decision rather than a log line ze wrote about itself.
//
// The denial is checked against its exact text because only authorization
// produces it. An unknown command, an unreachable daemon and a failed login all
// fail too, and each says something else. The localUser run that follows proves
// the same command IS reachable and allowed under another profile, so the
// denial cannot be an artifact of the command not existing.
func (l *scenarioLab) assertFilterIDProfile(ctx context.Context, password string) error {
	denied, err := l.login(ctx, radiusUser, password, deniedCommand)
	if err != nil {
		return err
	}
	if denied.ExitCode == 0 {
		return fmt.Errorf("%q was allowed for %s; the Access-Accept named the interop-operator profile, which denies it: %s",
			deniedCommand, radiusUser, denied.Output)
	}
	if !strings.Contains(denied.Output, unauthorizedMessage) {
		return fmt.Errorf("%q failed for %s without an authorization refusal (%q): %s",
			deniedCommand, radiusUser, unauthorizedMessage, denied.Output)
	}

	allowed, err := l.login(ctx, localUser, localPassword, deniedCommand)
	if err != nil {
		return err
	}
	if allowed.ExitCode != 0 {
		return fmt.Errorf("%q failed for the admin-profile control user %s (exit %d), so the refusal above proves nothing about the profile: %s",
			deniedCommand, localUser, allowed.ExitCode, allowed.Output)
	}
	return nil
}

// assertRejectStopsChain proves an Access-Reject ends the chain. The password
// it sends is the local account's real password, so a chain that fell through
// to local bcrypt would accept it.
func (l *scenarioLab) assertRejectStopsChain(ctx context.Context, password string, shape credentialShape, assertion int) error {
	result, err := l.login(ctx, radiusUser, password, allowedCommand)
	if err != nil {
		return fmt.Errorf("assertion %d: %w", assertion, err)
	}
	if result.ExitCode == 0 {
		return fmt.Errorf("assertion %d: %s logged in with the LOCAL password while RADIUS rejected it; the chain fell through: %s",
			assertion, radiusUser, result.Output)
	}
	if err := l.assertServerRecord(ctx, wantRecord(verdictReject, radiusUser, shape)); err != nil {
		return fmt.Errorf("assertion %d: %w", assertion, err)
	}
	if err := l.assertNoAuthSourceLocal(ctx, radiusUser); err != nil {
		return fmt.Errorf("assertion %d: %w", assertion, err)
	}
	return l.assertAuthFailureRecorded(ctx, radiusUser, assertion)
}

// assertPAPProbeAccepted asks the server directly, from inside its own
// container, whether the user file entry accepts the password over PAP.
func (l *scenarioLab) assertPAPProbeAccepted(ctx context.Context, user, password string) error {
	var tb textbuf.Buffer
	request := tb.Str("User-Name=").Str(user).Str(",User-Password=").Str(password).
		Str(",NAS-Identifier=").Str(probeNAS).String()
	// The probe dials the server's own lab address rather than the loopback,
	// because clients.conf authorizes the lab subnet and nothing else.
	script := tb.Reset().Str("echo ").Str(shellQuote(request)).
		Str(" | radclient -x 172.27.0.3:1812 auth ze-interop-fixture-secret").String()
	output, err := l.lab.Query(ctx, serverPeer, []string{"sh", "-c", script}, nil)
	if err != nil {
		return fmt.Errorf("radclient PAP control probe failed: %w", err)
	}
	if !strings.Contains(output, "Received Access-Accept") {
		return fmt.Errorf("the server refused the PAP control probe for %s, so its user entry is wrong and the CHAP rejection proves nothing: %s",
			user, output)
	}
	return nil
}

// assertServerRecord waits until the server's own log carries the exact line
// the scenario expects. A missing or unreadable file is a transient probe
// failure, so a wait that never measured the file reports that rather than
// reading absence as a verdict.
func (l *scenarioLab) assertServerRecord(ctx context.Context, want recordWant) error {
	var tb textbuf.Buffer
	description := tb.Str("FreeRADIUS record '").Str(want.String()).Byte('\'').String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     l.timeout,
		Interval:    time.Second,
		Description: description,
	}, func(probe context.Context) (string, error) {
		return l.lab.Query(probe, serverPeer, []string{"cat", serverLogPath}, nil)
	}, func(text string) bool {
		return serverRecorded(text, want)
	})
	return err
}

// assertAuthSource waits for ze's own log to report a successful SSH login for
// username satisfied by the named backend.
func (l *scenarioLab) assertAuthSource(ctx context.Context, username, source string) error {
	var tb textbuf.Buffer
	description := tb.Str("ze log 'SSH auth success' for ").Str(username).
		Str(" with source=").Str(source).String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     l.timeout,
		Interval:    time.Second,
		Description: description,
	}, l.readZeLog, func(text string) bool {
		return slices.Contains(authSuccessSources(text, username), source)
	})
	return err
}

// assertAuthFailureRecorded proves the refused attempt reached ze. Without it,
// the absence of a success line would also be satisfied by an attempt that was
// never made.
func (l *scenarioLab) assertAuthFailureRecorded(ctx context.Context, username string, assertion int) error {
	var tb textbuf.Buffer
	description := tb.Str("ze log 'SSH auth failure' for ").Str(username).String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     l.timeout,
		Interval:    time.Second,
		Description: description,
	}, l.readZeLog, func(text string) bool {
		return authFailures(text, username) > 0
	})
	if err != nil {
		return fmt.Errorf("assertion %d: %w", assertion, err)
	}
	return nil
}

// assertNoAuthSourceLocal refuses a login the local backend satisfied. It reads
// the log once, after the failure line above proved the log is being written.
func (l *scenarioLab) assertNoAuthSourceLocal(ctx context.Context, username string) error {
	text, err := l.readZeLog(ctx)
	if err != nil {
		return err
	}
	if slices.Contains(authSuccessSources(text, username), sourceLocal) {
		return fmt.Errorf("ze authenticated %s through the local backend after RADIUS rejected it; the chain did not stop", username)
	}
	return nil
}

// assertNoAuthSuccess refuses any successful login for username, whatever
// backend claimed it.
func (l *scenarioLab) assertNoAuthSuccess(ctx context.Context, username string) error {
	text, err := l.readZeLog(ctx)
	if err != nil {
		return err
	}
	if sources := authSuccessSources(text, username); len(sources) > 0 {
		return fmt.Errorf("ze authenticated %s through %v while the server rejected the request", username, sources)
	}
	return nil
}

func (l *scenarioLab) readZeLog(ctx context.Context) (string, error) {
	result, err := l.lab.Logs(ctx, zePeer, zeLogLines)
	if err != nil {
		return "", err
	}
	if !result.Available {
		return "", errors.New("ze container log could not be read")
	}
	return result.Text, nil
}

// login drives one operator login through ze's real SSH listener, from inside
// the ze container, exactly as `ze cli` does on a live box.
func (l *scenarioLab) login(ctx context.Context, user, password, command string) (loginResult, error) {
	result, err := l.lab.Exec(ctx, zePeer, []string{"sh", "-c", loginScript(user, password, command)}, nil)
	if err != nil {
		return loginResult{}, fmt.Errorf("run ze cli as %s: %w", user, err)
	}
	return parseLoginOutput(result.Stdout)
}

// loginScript seeds the CLI store once, then runs one command as the named
// user. The exit status is printed rather than returned, so the shell always
// exits 0 and a refused login can never be read as a Docker failure.
func loginScript(user, password, command string) string {
	var tb textbuf.Buffer
	tb.Str("store=").Str(cliStore).Byte('\n')
	tb.Str("if [ ! -d \"$store\" ]; then\n")
	tb.Str("  printf '%s\\n%s\\n127.0.0.1\\n%s\\n' ").Str(localUser).Byte(' ').
		Str(localPassword).Byte(' ').Str(sshPort).
		Str(" | ZE_CONFIG_DIR=\"$store\" ze init > \"$store.log\" 2>&1 || { echo ").
		Str(cliInitFailed).Str("; cat \"$store.log\"; exit 0; }\n")
	tb.Str("fi\n")
	tb.Str("ZE_CONFIG_DIR=\"$store\" ZE_SSH_PASSWORD=").Str(shellQuote(password)).
		Str(" ze cli --user ").Str(shellQuote(user)).Str(" -c ").Str(shellQuote(command)).Str(" 2>&1\n")
	tb.Str("echo \"").Str(cliExitMarker).Str("$?\"\n")
	return tb.String()
}

// parseLoginOutput reads the measured exit status out of the login output. An
// output with no marker is an error rather than a zero exit, because a missing
// measurement and a successful login are not the same answer.
func parseLoginOutput(output string) (loginResult, error) {
	if strings.Contains(output, cliInitFailed) {
		return loginResult{}, fmt.Errorf("the ze cli store could not be seeded: %s", output)
	}
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	for index, line := range slices.Backward(lines) {
		value, found := strings.CutPrefix(strings.TrimSpace(line), cliExitMarker)
		if !found {
			continue
		}
		code, err := strconv.Atoi(value)
		if err != nil {
			return loginResult{}, fmt.Errorf("ze cli exit marker %q is not a number", line)
		}
		body := strings.Join(lines[:index], "\n")
		return loginResult{Output: body, ExitCode: code}, nil
	}
	return loginResult{}, fmt.Errorf("ze cli produced no %s marker: %s", cliExitMarker, output)
}

// serverRecorded reports whether the server's log holds a line matching want.
func serverRecorded(text string, want recordWant) bool {
	for line := range strings.SplitSeq(text, "\n") {
		record, ok := parseServerRecord(line)
		if ok && want.matches(record) {
			return true
		}
	}
	return false
}

// parseServerRecord reads one linelog line. It answers false unless all six
// fields are present, so a truncated or reformatted line is never read as a
// partial verdict.
func parseServerRecord(line string) (serverRecord, bool) {
	fields := make(map[string]string, 6)
	for field := range strings.FieldsSeq(line) {
		key, value, found := strings.Cut(field, "=")
		if found {
			fields[key] = value
		}
	}
	record := serverRecord{
		Verdict:       fields["verdict"],
		User:          fields["user"],
		UserPassword:  fields["user-password"],
		CHAPPassword:  fields["chap-password"],
		EAPMessage:    fields["eap-message"],
		NASIdentifier: fields["nas-identifier"],
	}
	if record.Verdict == "" || record.User == "" || record.UserPassword == "" ||
		record.CHAPPassword == "" || record.EAPMessage == "" || record.NASIdentifier == "" {
		return serverRecord{}, false
	}
	return record, true
}

// authSuccessSources answers the source= value of every "SSH auth success" line
// naming username, in log order.
func authSuccessSources(text, username string) []string {
	sources := make([]string, 0, 2)
	for line := range strings.SplitSeq(text, "\n") {
		if !strings.Contains(line, "SSH auth success") {
			continue
		}
		if logField(line, "username") != username {
			continue
		}
		sources = append(sources, logField(line, "source"))
	}
	return sources
}

// authFailures counts the "SSH auth failure" lines naming username.
func authFailures(text, username string) int {
	count := 0
	for line := range strings.SplitSeq(text, "\n") {
		if !strings.Contains(line, "SSH auth failure") {
			continue
		}
		if logField(line, "username") == username {
			count++
		}
	}
	return count
}

// logField reads one key=value field out of a slog text line. The key must be
// preceded by a space, so username= never matches a longer key ending in it.
// An absent key answers the empty string, and every caller compares that
// against an expected value rather than acting on it.
func logField(line, key string) string {
	var tb textbuf.Buffer
	token := tb.Byte(' ').Str(key).Byte('=').String()
	_, rest, found := strings.Cut(line, token)
	if !found {
		return ""
	}
	if value, _, spaced := strings.Cut(rest, " "); spaced {
		return value
	}
	return rest
}

// shellQuote wraps a value in single quotes for a POSIX shell.
func shellQuote(value string) string {
	var tb textbuf.Buffer
	tb.Byte('\'')
	for index := range len(value) {
		if value[index] == '\'' {
			tb.Str("'\\''")
			continue
		}
		tb.Byte(value[index])
	}
	return tb.Byte('\'').String()
}
