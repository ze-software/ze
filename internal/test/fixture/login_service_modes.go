// Design: docs/architecture/aaa-tacacs.md -- remote backend, reload, recovery, and mode login-service functional fixtures.
// Related: register_login_service.go -- registers these fixture drivers.
// Related: login_service_helpers.go -- shared protocol clients and daemon helpers.
// Related: login_service_doors.go -- per-surface login-service functional fixtures.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/runner"
)

func lsFixtureTacacs(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("usage: plugin/login-service-tacacs <tacacs-port> <ssh-port> <web-port>")
	}
	tacPort, err := lsRequirePort(args[0])
	if err != nil {
		return err
	}
	sshPort, err := lsRequirePort(args[1])
	if err != nil {
		return err
	}
	webPort, err := lsRequirePort(args[2])
	if err != nil {
		return err
	}
	if err := pluginShellExtra4StartTacacs(ctx, tacPort, "tacssh:testpass:15", "--user", "tacweb:testpass:1"); err != nil {
		return err
	}
	workDir, err := os.Getwd()
	if err != nil {
		return err
	}
	var cfg textbuf.Buffer
	cfg.Str("system {\n\tauthentication {\n")
	// An unrestricted local user of the same name and password proves that the
	// TACACS+ login-service gate returns terminal ErrAuthRejected and never
	// falls through to local or opens the web fallback.
	cfg.Str("\t\tuser tacssh { password \"").Str(lsTestPasswordHash).Str("\"; profile [ admin ]; }\n")
	cfg.Str("\t\ttacacs {\n\t\t\tserver 127.0.0.1 { port ").Str(tacPort).Str("; key \"ze-mock-key\"; }\n\t\t\ttimeout 2;\n\t\t}\n")
	cfg.Str("\t\ttacacs-profile 15 { profile [ admin ]; login-service ssh-only; }\n")
	cfg.Str("\t\ttacacs-profile 1 { profile [ admin ]; login-service web-only; }\n")
	cfg.Str("\t}\n\tauthorization {\n")
	cfg.Str("\t\tprofile admin { run { default-action allow; } edit { default-action allow; } }\n")
	cfg.Str("\t\tlogin-service ssh-only { service [ ssh ]; }\n")
	cfg.Str("\t\tlogin-service web-only { service [ web ]; }\n")
	cfg.Str("\t}\n}\n")
	cfg.Str("environment {\n\tssh { enabled true; server main { ip 127.0.0.1; port ").Str(sshPort).Str("; } }\n")
	cfg.Str("\tweb { enabled true; server main { ip 127.0.0.1; port ").Str(webPort).Str("; } }\n}\n")

	cfgPath := filepath.Join(workDir, "tacacs-ls.conf")
	if err := lsWriteFile(cfgPath, cfg.String()); err != nil {
		return err
	}
	daemon, err := lsStartDaemon(ctx, workDir, cfgPath, nil)
	if err != nil {
		return err
	}
	defer daemon.stop()

	if err := lsWaitTCPReady(ctx, sshPort, "SSH"); err != nil {
		return fmt.Errorf("%w\n%s", err, daemon.logText())
	}
	if err := lsWaitTCPReady(ctx, webPort, "web"); err != nil {
		return fmt.Errorf("%w\n%s", err, daemon.logText())
	}
	client, closeIdle := lsNewHTTPSClient()
	defer closeIdle()

	// tacssh (priv-lvl 15 -> ssh-only): admitted on SSH, refused terminally on Web.
	if _, err := lsExecSSHPassword(sshPort, "tacssh", lsTestPassword, lsCmdShowVersion); err != nil {
		return fmt.Errorf("tacssh failed on its included SSH door: %w\n%s", err, daemon.logText())
	}
	refusedBasic, err := lsWebBasicGet(ctx, client, webPort, "tacssh", lsTestPassword, lsWebProbePath)
	if err != nil {
		return fmt.Errorf("tacssh web Basic request failed: %w", err)
	}
	if refusedBasic.status != http.StatusUnauthorized {
		return fmt.Errorf("tacssh on excluded web door returned HTTP %d, want 401 (must not fall through to local)", refusedBasic.status)
	}

	// tacweb (priv-lvl 1 -> web-only): admitted on Web, refused terminally on SSH.
	allowedBasic, err := lsWebBasicGet(ctx, client, webPort, "tacweb", lsTestPassword, lsWebProbePath)
	if err != nil {
		return fmt.Errorf("tacweb web Basic request failed: %w", err)
	}
	if allowedBasic.status != http.StatusOK {
		return fmt.Errorf("tacweb on included web door returned HTTP %d, want 200\n%s", allowedBasic.status, daemon.logText())
	}
	if sshClient, err := lsDialSSHPassword(sshPort, "tacweb", lsTestPassword); err == nil {
		_ = sshClient.Close() //nolint:errcheck // fixture cleanup
		return errors.New("tacweb with web-only tacacs-profile unexpectedly authenticated on SSH")
	}

	logText := daemon.logText()
	if !strings.Contains(logText, lsGateLogMsg) {
		return fmt.Errorf("daemon log missing %q:\n%s", lsGateLogMsg, logText)
	}
	fmt.Fprintln(os.Stderr, "OK: TACACS+ tacacs-profile login-service refused excluded doors terminally and admitted included doors")
	return nil
}

func lsFixtureRadius(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: plugin/login-service-radius <ssh-port> <web-port>")
	}
	sshPort, err := lsRequirePort(args[0])
	if err != nil {
		return err
	}
	webPort, err := lsRequirePort(args[1])
	if err != nil {
		return err
	}
	mock, mockAddr, err := extra1StartRadiusMock(ctx)
	if err != nil {
		return err
	}
	defer extra1StopRadiusMock(mock)
	_, mockPort, err := net.SplitHostPort(mockAddr)
	if err != nil {
		return fmt.Errorf("parse RADIUS mock addr %q: %w", mockAddr, err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		return err
	}
	var cfg textbuf.Buffer
	cfg.Str("system {\n\tauthentication {\n")
	// Unrestricted local admin with the same password proves that a RADIUS
	// login-service refusal is terminal and never falls through to local/web fallback.
	cfg.Str("\t\tuser admin { password \"").Str(lsTestPasswordHash).Str("\"; profile [ admin ]; }\n")
	cfg.Str("\t\tradius {\n\t\t\tserver 127.0.0.1 { port ").Str(mockPort).Str("; key \"ze-mock-key\"; }\n")
	cfg.Str("\t\t\ttimeout 2;\n\t\t\tretries 1;\n\t\t\tlogin-service ssh-only;\n\t\t}\n")
	cfg.Str("\t}\n\tauthorization {\n")
	cfg.Str("\t\tprofile admin { run { default-action allow; } edit { default-action allow; } }\n")
	cfg.Str("\t\tlogin-service ssh-only { service [ ssh ]; }\n")
	cfg.Str("\t}\n}\n")
	cfg.Str("environment {\n\tssh { enabled true; server main { ip 127.0.0.1; port ").Str(sshPort).Str("; } }\n")
	cfg.Str("\tweb { enabled true; server main { ip 127.0.0.1; port ").Str(webPort).Str("; } }\n}\n")

	cfgPath := filepath.Join(workDir, "radius-ls.conf")
	if err := lsWriteFile(cfgPath, cfg.String()); err != nil {
		return err
	}
	daemon, err := lsStartDaemon(ctx, workDir, cfgPath, nil)
	if err != nil {
		return err
	}
	defer daemon.stop()

	if err := lsWaitTCPReady(ctx, sshPort, "SSH"); err != nil {
		return fmt.Errorf("%w\n%s", err, daemon.logText())
	}
	if err := lsWaitTCPReady(ctx, webPort, "web"); err != nil {
		return fmt.Errorf("%w\n%s", err, daemon.logText())
	}
	client, closeIdle := lsNewHTTPSClient()
	defer closeIdle()

	// Included door: RADIUS admin succeeds on SSH with source=radius.
	if _, err := lsExecSSHPassword(sshPort, lsUserAdmin, lsTestPassword, lsCmdShowVersion); err != nil {
		return fmt.Errorf("RADIUS admin failed on included SSH door: %w\n%s", err, daemon.logText())
	}
	// Excluded door: RADIUS admin is refused terminally on Web (HTTP 401).
	webResp, err := lsWebBasicGet(ctx, client, webPort, lsUserAdmin, lsTestPassword, lsWebProbePath)
	if err != nil {
		return fmt.Errorf("RADIUS admin web request failed: %w", err)
	}
	if webResp.status != http.StatusUnauthorized {
		return fmt.Errorf("RADIUS admin on excluded web door returned HTTP %d, want 401 (must not fall through to local)", webResp.status)
	}

	logText := daemon.logText()
	if !strings.Contains(logText, "source=radius") {
		return fmt.Errorf("daemon log missing source=radius:\n%s", logText)
	}
	if !strings.Contains(logText, lsGateLogMsg) {
		return fmt.Errorf("daemon log missing %q:\n%s", lsGateLogMsg, logText)
	}
	mockLog := mock.contents()
	if !strings.Contains(mockLog, "nas-port-id=ssh") {
		return fmt.Errorf("RADIUS mock log missing unchanged NAS-Port-Id ssh:\n%s", mockLog)
	}
	if !strings.Contains(mockLog, "nas-port-id=web") {
		return fmt.Errorf("RADIUS mock log missing unchanged NAS-Port-Id web:\n%s", mockLog)
	}
	fmt.Fprintln(os.Stderr, "OK: RADIUS login-service refused web terminally, admitted ssh, and preserved NAS-Port-Id")
	return nil
}

func lsFixtureReloadTightens(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: plugin/login-service-reload-tightens <ssh-port> <rest-port>")
	}
	sshPort, err := lsRequirePort(args[0])
	if err != nil {
		return err
	}
	restPort, err := lsRequirePort(args[1])
	if err != nil {
		return err
	}
	workDir, err := os.Getwd()
	if err != nil {
		return err
	}
	cfgPath := filepath.Join(workDir, "reload-tightens.conf")
	buildCfg := func(services string) string {
		var tb textbuf.Buffer
		tb.Str("system {\n\tauthentication {\n")
		tb.Str("\t\tuser alice { password \"").Str(lsTestPasswordHash).Str("\"; login-service ops; }\n")
		tb.Str("\t}\n\tauthorization {\n")
		tb.Str("\t\tlogin-service ops { service [ ").Str(services).Str(" ]; }\n")
		tb.Str("\t}\n}\n")
		tb.Str("environment {\n\tssh { enabled true; server main { ip 127.0.0.1; port ").Str(sshPort).Str("; } }\n")
		tb.Str("\tapi-server { rest { enabled true; server main { ip 127.0.0.1; port ").Str(restPort).Str("; } } }\n}\n")
		return tb.String()
	}
	if err := lsWriteFile(cfgPath, buildCfg("ssh rest")); err != nil {
		return err
	}
	daemon, err := lsStartDaemon(ctx, workDir, cfgPath, nil)
	if err != nil {
		return err
	}
	defer daemon.stop()

	if err := lsWaitSSHAndRESTReady(ctx, sshPort, restPort); err != nil {
		return fmt.Errorf("%w\n%s", err, daemon.logText())
	}

	// Open an SSH connection BEFORE reload and prove it works.
	openClient, err := lsDialSSHPassword(sshPort, lsUserAlice, lsTestPassword)
	if err != nil {
		return fmt.Errorf("pre-reload SSH login failed: %w\n%s", err, daemon.logText())
	}
	defer openClient.Close() //nolint:errcheck // fixture cleanup
	if _, err := lsRunSSHCommand(openClient, lsCmdShowVersion); err != nil {
		return fmt.Errorf("pre-reload open SSH command failed: %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK: pre-reload SSH connection opened and answered")

	// Reload tightening ops from [ ssh rest ] to [ rest ].
	if err := lsWriteFile(cfgPath, buildCfg("rest")); err != nil {
		return err
	}
	if err := daemon.reloadAndWait(ctx, 1); err != nil {
		return err
	}

	// AC-13: already-open SSH connection keeps answering.
	if _, err := lsRunSSHCommand(openClient, lsCmdShowVersion); err != nil {
		return fmt.Errorf("already-open SSH connection failed after reload: %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK: already-open SSH connection kept answering after reload")

	// AC-5: a NEW SSH login is refused.
	if newClient, err := lsDialSSHPassword(sshPort, lsUserAlice, lsTestPassword); err == nil {
		_ = newClient.Close() //nolint:errcheck // fixture cleanup
		return errors.New("new SSH login unexpectedly succeeded after reload withdrew ssh")
	}
	fmt.Fprintln(os.Stderr, "OK: new SSH login refused after reload withdrew ssh")

	// AC-5: REST remains admitted after reload.
	restCode, restBody, err := lsRESTExecute(ctx, restPort, lsUserAlice+":"+lsTestPassword, lsCmdShowVersion)
	if err != nil {
		return fmt.Errorf("REST failed after reload: %w", err)
	}
	if restCode != http.StatusOK {
		return fmt.Errorf("REST returned HTTP %d after reload, want 200: %s", restCode, restBody)
	}
	fmt.Fprintln(os.Stderr, "OK: REST remained admitted after reload tightened SSH")
	return nil
}

func lsFixtureRecoveryUser(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("usage: plugin/login-service-recovery-user <ssh-port> <web-port> <rest-port>")
	}
	sshPort, err := lsRequirePort(args[0])
	if err != nil {
		return err
	}
	webPort, err := lsRequirePort(args[1])
	if err != nil {
		return err
	}
	restPort, err := lsRequirePort(args[2])
	if err != nil {
		return err
	}
	restPortNum, err := strconv.Atoi(restPort)
	if err != nil {
		return err
	}
	// Reserve a 4th port for gRPC; defer Release BEFORE daemon.stop so daemon stops first.
	reserved, _, err := runner.ReservePorts(restPortNum+1, 1)
	if err != nil {
		return fmt.Errorf("reserve gRPC port: %w", err)
	}
	defer reserved.Release()
	grpcPort := strconv.Itoa(reserved.Start)

	workDir, err := os.Getwd()
	if err != nil {
		return err
	}
	// Seed the stored recovery user "recovery" in workDir's database/.
	if err := lsInitStore(ctx, workDir, "recovery", lsRecoveryPassword, sshPort); err != nil {
		return err
	}
	var cfg textbuf.Buffer
	cfg.Str("system {\n\tauthentication {\n")
	cfg.Str("\t\tuser ordinary { password \"").Str(lsTestPasswordHash).Str("\"; login-service none-at-all; }\n")
	cfg.Str("\t}\n\tauthorization {\n")
	cfg.Str("\t\tprofile locked { run { default-action deny; } edit { default-action deny; } }\n")
	cfg.Str("\t\tlogin-service none-at-all { }\n")
	cfg.Str("\t}\n}\n")
	cfg.Str("environment {\n")
	cfg.Str("\tssh { enabled true; server main { ip 127.0.0.1; port ").Str(sshPort).Str("; } }\n")
	cfg.Str("\tweb { enabled true; server main { ip 127.0.0.1; port ").Str(webPort).Str("; } }\n")
	cfg.Str("\tapi-server {\n")
	cfg.Str("\t\trest { enabled true; server main { ip 127.0.0.1; port ").Str(restPort).Str("; } }\n")
	cfg.Str("\t\tgrpc { enabled true; server main { ip 127.0.0.1; port ").Str(grpcPort).Str("; } }\n")
	cfg.Str("\t}\n}\n")

	cfgPath := filepath.Join(workDir, "recovery-ls.conf")
	if err := lsWriteFile(cfgPath, cfg.String()); err != nil {
		return err
	}
	daemon, err := lsStartDaemon(ctx, workDir, cfgPath, map[string]string{envConfigDir: workDir})
	if err != nil {
		return err
	}
	defer daemon.stop()

	for _, pair := range [][2]string{{sshPort, "SSH"}, {webPort, "web"}, {restPort, "REST"}, {grpcPort, "gRPC"}} {
		if err := lsWaitTCPReady(ctx, pair[0], pair[1]); err != nil {
			return fmt.Errorf("%w\n%s", err, daemon.logText())
		}
	}
	webClient, closeIdle := lsNewHTTPSClient()
	defer closeIdle()
	grpcConn, err := lsNewGRPCClient(grpcPort)
	if err != nil {
		return err
	}
	defer grpcConn.Close() //nolint:errcheck // fixture cleanup

	// Ordinary config account MUST be refused on every door.
	if err := lsAssertUserRefusedAllDoors(ctx, sshPort, webPort, restPort, webClient, grpcConn, "ordinary", lsTestPassword); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: ordinary config account refused on SSH, Web, REST, and gRPC")

	// Stored recovery user MUST be admitted on SSH, ze cli, Web (Basic + form), REST, and gRPC (AC-8).
	if err := lsAssertUserAdmittedAllDoors(ctx, sshPort, webPort, restPort, webClient, grpcConn, "recovery", lsRecoveryPassword); err != nil {
		return fmt.Errorf("%w\n%s", err, daemon.logText())
	}
	formResp, err := lsWebFormLogin(ctx, webClient, webPort, "recovery", lsRecoveryPassword)
	if err != nil {
		return fmt.Errorf("recovery web form login failed: %w", err)
	}
	if formResp.status != http.StatusSeeOther {
		return fmt.Errorf("recovery web form status=%d, want 303", formResp.status)
	}
	if formResp.cookie == "" {
		return errors.New("recovery web form login issued no session cookie")
	}
	cliOut, cliErr := lsRunCLI(ctx, workDir, sshPort, "recovery", lsRecoveryPassword, lsCmdShowVersion)
	if cliErr != nil {
		return fmt.Errorf("recovery user ze cli failed: %w\n%s", cliErr, cliOut)
	}
	fmt.Fprintln(os.Stderr, "OK: stored recovery user authenticated on SSH, ze cli, Web, REST, and gRPC")
	return nil
}

func lsFixtureShadowedRecoveryName(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("usage: plugin/login-service-shadowed-recovery-name <ssh-port> <rest-port> <web-port>")
	}
	sshPort, err := lsRequirePort(args[0])
	if err != nil {
		return err
	}
	restPort, err := lsRequirePort(args[1])
	if err != nil {
		return err
	}
	webPort, err := lsRequirePort(args[2])
	if err != nil {
		return err
	}
	workDir, err := os.Getwd()
	if err != nil {
		return err
	}
	// Seed stored recovery user named "admin".
	if err := lsInitStore(ctx, workDir, lsUserAdmin, lsRecoveryPassword, sshPort); err != nil {
		return err
	}
	// Shadow "admin" in config with a different password and login-service rest-only (OD-2 option a).
	var cfg textbuf.Buffer
	cfg.Str("system {\n\tauthentication {\n")
	cfg.Str("\t\tuser admin { password \"").Str(lsTestPasswordHash).Str("\"; login-service rest-only; }\n")
	cfg.Str("\t}\n\tauthorization {\n")
	cfg.Str("\t\tlogin-service rest-only { service [ rest ]; }\n")
	cfg.Str("\t}\n}\n")
	cfg.Str("environment {\n")
	cfg.Str("\tssh { enabled true; server main { ip 127.0.0.1; port ").Str(sshPort).Str("; } }\n")
	cfg.Str("\tweb { enabled true; server main { ip 127.0.0.1; port ").Str(webPort).Str("; } }\n")
	cfg.Str("\tapi-server { rest { enabled true; server main { ip 127.0.0.1; port ").Str(restPort).Str("; } } }\n}\n")

	cfgPath := filepath.Join(workDir, "shadowed-recovery.conf")
	if err := lsWriteFile(cfgPath, cfg.String()); err != nil {
		return err
	}
	daemon, err := lsStartDaemon(ctx, workDir, cfgPath, map[string]string{envConfigDir: workDir})
	if err != nil {
		return err
	}
	defer daemon.stop()

	for _, pair := range [][2]string{{sshPort, "SSH"}, {restPort, "REST"}, {webPort, "web"}} {
		if err := lsWaitTCPReady(ctx, pair[0], pair[1]); err != nil {
			return fmt.Errorf("%w\n%s", err, daemon.logText())
		}
	}
	webClient, closeIdle := lsNewHTTPSClient()
	defer closeIdle()

	// Stored recovery password must NOT work once shadowed.
	if client, err := lsDialSSHPassword(sshPort, lsUserAdmin, lsRecoveryPassword); err == nil {
		_ = client.Close() //nolint:errcheck // fixture cleanup
		return errors.New("shadowed stored recovery password unexpectedly authenticated on SSH")
	}

	// Shadowing config user is gated as an ordinary account: refused on SSH and Web.
	if client, err := lsDialSSHPassword(sshPort, lsUserAdmin, lsTestPassword); err == nil {
		_ = client.Close() //nolint:errcheck // fixture cleanup
		return errors.New("shadowing config user admin unexpectedly bypassed login-service gate on SSH")
	}
	webResp, err := lsWebBasicGet(ctx, webClient, webPort, lsUserAdmin, lsTestPassword, lsWebProbePath)
	if err != nil {
		return fmt.Errorf("shadowed admin web request failed: %w", err)
	}
	if webResp.status != http.StatusUnauthorized {
		return fmt.Errorf("shadowing config user admin on excluded web door returned HTTP %d, want 401", webResp.status)
	}

	// Shadowing config user IS admitted on its included door (REST).
	restCode, restBody, err := lsRESTExecute(ctx, restPort, lsUserAdmin+":"+lsTestPassword, lsCmdShowVersion)
	if err != nil {
		return fmt.Errorf("shadowing config user admin failed on included REST door: %w", err)
	}
	if restCode != http.StatusOK {
		return fmt.Errorf("shadowing config user admin on REST returned HTTP %d, want 200: %s", restCode, restBody)
	}
	fmt.Fprintln(os.Stderr, "OK: shadowing config user gated as ordinary account on SSH/Web and admitted on REST")
	return nil
}

func lsFixtureAbsentSection(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("usage: plugin/login-service-absent-section <ssh-port> <web-port> <rest-port>")
	}
	sshPort, err := lsRequirePort(args[0])
	if err != nil {
		return err
	}
	webPort, err := lsRequirePort(args[1])
	if err != nil {
		return err
	}
	restPort, err := lsRequirePort(args[2])
	if err != nil {
		return err
	}
	restPortNum, err := strconv.Atoi(restPort)
	if err != nil {
		return err
	}
	reserved, _, err := runner.ReservePorts(restPortNum+1, 1)
	if err != nil {
		return fmt.Errorf("reserve gRPC port: %w", err)
	}
	defer reserved.Release()
	grpcPort := strconv.Itoa(reserved.Start)

	workDir, err := os.Getwd()
	if err != nil {
		return err
	}
	// Config with NO login-service section at all (OD-3 option a / AC-7).
	var cfg textbuf.Buffer
	cfg.Str("system {\n\tauthentication {\n")
	cfg.Str("\t\tuser legacy { password \"").Str(lsTestPasswordHash).Str("\"; }\n")
	cfg.Str("\t}\n}\n")
	cfg.Str("environment {\n")
	cfg.Str("\tssh { enabled true; server main { ip 127.0.0.1; port ").Str(sshPort).Str("; } }\n")
	cfg.Str("\tweb { enabled true; server main { ip 127.0.0.1; port ").Str(webPort).Str("; } }\n")
	cfg.Str("\tapi-server {\n")
	cfg.Str("\t\trest { enabled true; server main { ip 127.0.0.1; port ").Str(restPort).Str("; } }\n")
	cfg.Str("\t\tgrpc { enabled true; server main { ip 127.0.0.1; port ").Str(grpcPort).Str("; } }\n")
	cfg.Str("\t}\n}\n")

	cfgPath := filepath.Join(workDir, "absent-section.conf")
	if err := lsWriteFile(cfgPath, cfg.String()); err != nil {
		return err
	}
	daemon, err := lsStartDaemon(ctx, workDir, cfgPath, nil)
	if err != nil {
		return err
	}
	defer daemon.stop()

	for _, pair := range [][2]string{{sshPort, "SSH"}, {webPort, "web"}, {restPort, "REST"}, {grpcPort, "gRPC"}} {
		if err := lsWaitTCPReady(ctx, pair[0], pair[1]); err != nil {
			return fmt.Errorf("%w\n%s", err, daemon.logText())
		}
	}
	webClient, closeIdle := lsNewHTTPSClient()
	defer closeIdle()
	grpcConn, err := lsNewGRPCClient(grpcPort)
	if err != nil {
		return err
	}
	defer grpcConn.Close() //nolint:errcheck // fixture cleanup

	// Wrong password is refused everywhere (proving auth is active).
	if err := lsAssertUserRefusedAllDoors(ctx, sshPort, webPort, restPort, webClient, grpcConn, "legacy", lsWrongPassword); err != nil {
		return err
	}
	// Correct password is admitted on every door.
	if err := lsAssertUserAdmittedAllDoors(ctx, sshPort, webPort, restPort, webClient, grpcConn, "legacy", lsTestPassword); err != nil {
		return fmt.Errorf("%w\n%s", err, daemon.logText())
	}
	fmt.Fprintln(os.Stderr, "OK: config without login-service section admitted user on SSH, Web, REST, and gRPC")
	return nil
}

func lsFixtureEmptySet(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("usage: plugin/login-service-empty-set <ssh-port> <web-port> <rest-port>")
	}
	sshPort, err := lsRequirePort(args[0])
	if err != nil {
		return err
	}
	webPort, err := lsRequirePort(args[1])
	if err != nil {
		return err
	}
	restPort, err := lsRequirePort(args[2])
	if err != nil {
		return err
	}
	restPortNum, err := strconv.Atoi(restPort)
	if err != nil {
		return err
	}
	reserved, _, err := runner.ReservePorts(restPortNum+1, 1)
	if err != nil {
		return fmt.Errorf("reserve gRPC port: %w", err)
	}
	defer reserved.Release()
	grpcPort := strconv.Itoa(reserved.Start)

	workDir, err := os.Getwd()
	if err != nil {
		return err
	}
	// AC-11: empty login-service set refuses every gated door; absent reference admits all.
	var cfg textbuf.Buffer
	cfg.Str("system {\n\tauthentication {\n")
	cfg.Str("\t\tuser locked { password \"").Str(lsTestPasswordHash).Str("\"; login-service blocked; }\n")
	cfg.Str("\t\tuser open { password \"").Str(lsTestPasswordHash).Str("\"; }\n")
	cfg.Str("\t}\n\tauthorization {\n")
	cfg.Str("\t\tlogin-service blocked { }\n")
	cfg.Str("\t}\n}\n")
	cfg.Str("environment {\n")
	cfg.Str("\tssh { enabled true; server main { ip 127.0.0.1; port ").Str(sshPort).Str("; } }\n")
	cfg.Str("\tweb { enabled true; server main { ip 127.0.0.1; port ").Str(webPort).Str("; } }\n")
	cfg.Str("\tapi-server {\n")
	cfg.Str("\t\trest { enabled true; server main { ip 127.0.0.1; port ").Str(restPort).Str("; } }\n")
	cfg.Str("\t\tgrpc { enabled true; server main { ip 127.0.0.1; port ").Str(grpcPort).Str("; } }\n")
	cfg.Str("\t}\n}\n")

	cfgPath := filepath.Join(workDir, "empty-set.conf")
	if err := lsWriteFile(cfgPath, cfg.String()); err != nil {
		return err
	}
	daemon, err := lsStartDaemon(ctx, workDir, cfgPath, nil)
	if err != nil {
		return err
	}
	defer daemon.stop()

	for _, pair := range [][2]string{{sshPort, "SSH"}, {webPort, "web"}, {restPort, "REST"}, {grpcPort, "gRPC"}} {
		if err := lsWaitTCPReady(ctx, pair[0], pair[1]); err != nil {
			return fmt.Errorf("%w\n%s", err, daemon.logText())
		}
	}
	webClient, closeIdle := lsNewHTTPSClient()
	defer closeIdle()
	grpcConn, err := lsNewGRPCClient(grpcPort)
	if err != nil {
		return err
	}
	defer grpcConn.Close() //nolint:errcheck // fixture cleanup

	if err := lsAssertUserRefusedAllDoors(ctx, sshPort, webPort, restPort, webClient, grpcConn, "locked", lsTestPassword); err != nil {
		return err
	}
	if err := lsAssertUserAdmittedAllDoors(ctx, sshPort, webPort, restPort, webClient, grpcConn, "open", lsTestPassword); err != nil {
		return fmt.Errorf("%w\n%s", err, daemon.logText())
	}
	fmt.Fprintln(os.Stderr, "OK: empty login-service set refused every door while unrestricted control succeeded everywhere")
	return nil
}

func lsFixtureChainTerminal(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("usage: plugin/login-service-chain-terminal <ssh-port> <web-port> <dead-tacacs-port>")
	}
	sshPort, err := lsRequirePort(args[0])
	if err != nil {
		return err
	}
	webPort, err := lsRequirePort(args[1])
	if err != nil {
		return err
	}
	deadPort, err := lsRequirePort(args[2])
	if err != nil {
		return err
	}
	workDir, err := os.Getwd()
	if err != nil {
		return err
	}
	// Dead TACACS+ server is configured (nothing listens on deadPort); local
	// user localnoweb excludes web, localweb allows web.
	var cfg textbuf.Buffer
	cfg.Str("system {\n\tauthentication {\n")
	cfg.Str("\t\tuser localnoweb { password \"").Str(lsTestPasswordHash).Str("\"; login-service ssh-only; }\n")
	cfg.Str("\t\tuser localweb { password \"").Str(lsTestPasswordHash).Str("\"; login-service web-only; }\n")
	cfg.Str("\t\ttacacs {\n\t\t\tserver 127.0.0.1 { port ").Str(deadPort).Str("; key \"ze-mock-key\"; }\n\t\t\ttimeout 1;\n\t\t}\n")
	cfg.Str("\t}\n\tauthorization {\n")
	cfg.Str("\t\tlogin-service ssh-only { service [ ssh ]; }\n")
	cfg.Str("\t\tlogin-service web-only { service [ web ]; }\n")
	cfg.Str("\t}\n}\n")
	cfg.Str("environment {\n")
	cfg.Str("\tssh { enabled true; server main { ip 127.0.0.1; port ").Str(sshPort).Str("; } }\n")
	cfg.Str("\tweb { enabled true; server main { ip 127.0.0.1; port ").Str(webPort).Str("; } }\n}\n")

	cfgPath := filepath.Join(workDir, "chain-terminal.conf")
	if err := lsWriteFile(cfgPath, cfg.String()); err != nil {
		return err
	}
	daemon, err := lsStartDaemon(ctx, workDir, cfgPath, nil)
	if err != nil {
		return err
	}
	defer daemon.stop()

	if err := lsWaitTCPReady(ctx, sshPort, "SSH"); err != nil {
		return fmt.Errorf("%w\n%s", err, daemon.logText())
	}
	if err := lsWaitTCPReady(ctx, webPort, "web"); err != nil {
		return fmt.Errorf("%w\n%s", err, daemon.logText())
	}
	webClient, closeIdle := lsNewHTTPSClient()
	defer closeIdle()

	// Excluded local account on web form: TACACS+ is unreachable, local authenticates,
	// login-service gate returns terminal ErrAuthRejected, and web fallback MUST NOT admit it.
	formResp, err := lsWebFormLogin(ctx, webClient, webPort, "localnoweb", lsTestPassword)
	if err != nil {
		return fmt.Errorf("localnoweb web form failed: %w", err)
	}
	if formResp.status != http.StatusUnauthorized {
		return fmt.Errorf("localnoweb on web form returned HTTP %d, want 401", formResp.status)
	}
	if formResp.cookie != "" {
		return errors.New("localnoweb on web form unexpectedly received a session cookie")
	}

	// Gate refusal must be logged ONCE for this attempt (AC-15: web fallback did not re-run/admit).
	logText := daemon.logText()
	refusalLines := 0
	for line := range strings.SplitSeq(logText, "\n") {
		if !strings.Contains(line, lsGateLogMsg) {
			continue
		}
		if !strings.Contains(line, "account=localnoweb") {
			continue
		}
		if !strings.Contains(line, "service=web") {
			continue
		}
		refusalLines++
	}
	if refusalLines != 1 {
		return fmt.Errorf("expected exactly 1 login-service gate refusal log line for localnoweb/web, got %d:\n%s", refusalLines, logText)
	}

	// Same excluded account succeeds on SSH (after TACACS+ unreachable fallback to local).
	if _, err := lsExecSSHPassword(sshPort, "localnoweb", lsTestPassword, lsCmdShowVersion); err != nil {
		return fmt.Errorf("localnoweb failed on included SSH door: %w\n%s", err, daemon.logText())
	}
	// Control account succeeds on Web form.
	okForm, err := lsWebFormLogin(ctx, webClient, webPort, "localweb", lsTestPassword)
	if err != nil {
		return fmt.Errorf("localweb failed on included web door: %w", err)
	}
	if okForm.status != http.StatusSeeOther {
		return fmt.Errorf("localweb on web form returned HTTP %d, want 303", okForm.status)
	}
	if okForm.cookie == "" {
		return errors.New("localweb on web form received no session cookie")
	}
	fmt.Fprintln(os.Stderr, "OK: unreachable remote backend fell through to local and web gate refusal stayed terminal")
	return nil
}

func lsFixtureSharedToken(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: plugin/login-service-shared-token <rest-port> <grpc-port>")
	}
	restPort, err := lsRequirePort(args[0])
	if err != nil {
		return err
	}
	grpcPort, err := lsRequirePort(args[1])
	if err != nil {
		return err
	}
	if err := lsWaitTCPReady(ctx, restPort, "REST"); err != nil {
		return err
	}
	if err := lsWaitTCPReady(ctx, grpcPort, "gRPC"); err != nil {
		return err
	}
	grpcConn, err := lsNewGRPCClient(grpcPort)
	if err != nil {
		return err
	}
	defer grpcConn.Close() //nolint:errcheck // fixture cleanup

	// Wrong token must be refused on both REST and gRPC.
	badREST, _, err := lsRESTExecute(ctx, restPort, "wrong-shared-token", lsCmdShowVersion)
	if err != nil {
		return fmt.Errorf("wrong shared token on REST request failed: %w", err)
	}
	if badREST != http.StatusUnauthorized {
		return fmt.Errorf("wrong shared token on REST returned HTTP %d, want 401", badREST)
	}
	_, badGRPC, _, _ := lsGRPCExecute(ctx, grpcConn, "wrong-shared-token", lsCmdShowVersion)
	if badGRPC != codes.Unauthenticated {
		return fmt.Errorf("wrong shared token on gRPC returned code=%s, want Unauthenticated", badGRPC)
	}

	// Correct shared token MUST be admitted on both REST and gRPC (AC-16).
	okREST, restBody, err := lsRESTExecute(ctx, restPort, "shared-api-secret", lsCmdShowVersion)
	if err != nil {
		return fmt.Errorf("shared token on REST failed: %w", err)
	}
	if okREST != http.StatusOK {
		return fmt.Errorf("shared token on REST returned HTTP %d, want 200: %s", okREST, restBody)
	}
	okGRPC, grpcCode, grpcMsg, err := lsGRPCExecute(ctx, grpcConn, "shared-api-secret", lsCmdShowVersion)
	if err != nil {
		return fmt.Errorf("shared token on gRPC failed: code=%s msg=%q err=%w", grpcCode, grpcMsg, err)
	}
	if grpcCode != codes.OK {
		return fmt.Errorf("shared token on gRPC code=%s msg=%q, want OK", grpcCode, grpcMsg)
	}
	if okGRPC.GetStatus() != statusDone {
		return fmt.Errorf("shared token on gRPC status=%q, want done", okGRPC.GetStatus())
	}
	fmt.Fprintln(os.Stderr, "OK: shared-token mode stayed admitted on REST and gRPC beside login-service sets")
	return nil
}

func lsAssertUserRefusedAllDoors(
	ctx context.Context,
	sshPort, webPort, restPort string,
	webClient *http.Client,
	grpcConn *grpc.ClientConn,
	username, password string,
) error {
	if client, err := lsDialSSHPassword(sshPort, username, password); err == nil {
		_ = client.Close() //nolint:errcheck // fixture cleanup
		return fmt.Errorf("%s unexpectedly authenticated on SSH", username)
	}
	webResp, err := lsWebBasicGet(ctx, webClient, webPort, username, password, lsWebProbePath)
	if err != nil {
		return fmt.Errorf("%s web Basic probe failed: %w", username, err)
	}
	if webResp.status != http.StatusUnauthorized {
		return fmt.Errorf("%s on web returned HTTP %d, want 401", username, webResp.status)
	}
	restCode, _, err := lsRESTExecute(ctx, restPort, username+":"+password, lsCmdShowVersion)
	if err != nil {
		return fmt.Errorf("%s REST probe failed: %w", username, err)
	}
	if restCode != http.StatusUnauthorized {
		return fmt.Errorf("%s on REST returned HTTP %d, want 401", username, restCode)
	}
	_, grpcCode, _, _ := lsGRPCExecute(ctx, grpcConn, username+":"+password, lsCmdShowVersion)
	if grpcCode != codes.Unauthenticated {
		return fmt.Errorf("%s on gRPC returned code=%s, want Unauthenticated", username, grpcCode)
	}
	return nil
}

func lsAssertUserAdmittedAllDoors(
	ctx context.Context,
	sshPort, webPort, restPort string,
	webClient *http.Client,
	grpcConn *grpc.ClientConn,
	username, password string,
) error {
	if _, err := lsExecSSHPassword(sshPort, username, password, lsCmdShowVersion); err != nil {
		return fmt.Errorf("%s failed on SSH: %w", username, err)
	}
	webResp, err := lsWebBasicGet(ctx, webClient, webPort, username, password, lsWebProbePath)
	if err != nil {
		return fmt.Errorf("%s web Basic failed: %w", username, err)
	}
	if webResp.status != http.StatusOK {
		return fmt.Errorf("%s on web returned HTTP %d, want 200", username, webResp.status)
	}
	restCode, restBody, err := lsRESTExecute(ctx, restPort, username+":"+password, lsCmdShowVersion)
	if err != nil {
		return fmt.Errorf("%s REST failed: %w", username, err)
	}
	if restCode != http.StatusOK {
		return fmt.Errorf("%s on REST returned HTTP %d, want 200: %s", username, restCode, restBody)
	}
	grpcResp, grpcCode, grpcMsg, err := lsGRPCExecute(ctx, grpcConn, username+":"+password, lsCmdShowVersion)
	if err != nil {
		return fmt.Errorf("%s gRPC failed: code=%s msg=%q err=%w", username, grpcCode, grpcMsg, err)
	}
	if grpcCode != codes.OK {
		return fmt.Errorf("%s gRPC returned code=%s msg=%q", username, grpcCode, grpcMsg)
	}
	if grpcResp.GetStatus() != statusDone {
		return fmt.Errorf("%s gRPC returned status=%q, want done", username, grpcResp.GetStatus())
	}
	return nil
}
