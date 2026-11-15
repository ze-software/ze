// Design: docs/architecture/firewall/firewall-domain-group.md -- a start asks only for the answers that expired
// Related: netfilter_fixture_domain_group.go -- the DNS stub and kernel read-back this driver shares;
// plugin_shell_extra_1.go -- extra1StartDaemon, the daemon launcher it starts twice;
// register_domain_group.go -- where its name is registered

package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	mdns "github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/textbuf"
	dnsmock "github.com/ze-software/ze/internal/test/mock/dns"
)

const (
	// The two groups of the restart scenario. alpha's floor is an hour, so
	// both of its families carry a stored expiry an hour out whatever TTL the
	// stub's empty AAAA answer reports. zulu's floor is one second, so its
	// stored answer has expired by the time the daemon starts again. alpha
	// sorts before zulu (lessKey, internal/component/firewall/plugins/domain/
	// schedule.go), so a start that made every unit due would ask for alpha
	// first, before the zulu answer this driver waits for.
	domainGroupRestartLong      = "alpha"
	domainGroupRestartLongName  = "alpha.example.test"
	domainGroupRestartShort     = "zulu"
	domainGroupRestartShortName = "zulu.example.test"

	// What each name answers before the restart, and after it. An address
	// from the second pair in a set says the name was asked again.
	domainGroupRestartLongFirst   = "203.0.113.20"
	domainGroupRestartLongSecond  = "203.0.113.29"
	domainGroupRestartShortFirst  = "198.51.100.20"
	domainGroupRestartShortSecond = "198.51.100.29"

	// domainGroupRestartLongTTL is the answer TTL alpha carries, the hour
	// AC-13 describes. The group floor of an hour is what makes the empty AAAA
	// family wait as long.
	domainGroupRestartLongTTL = 3600

	// domainGroupRestartTable is the scenario's table, distinct from the other
	// three scenarios in the dns-stub-port-53 group.
	domainGroupRestartTable = "fw_dgr"

	// domainGroupRestartPollAttempts bounds each kernel read-back that spans a
	// daemon start. At netfilterPollDelay it is twenty seconds.
	domainGroupRestartPollAttempts = 400

	// domainGroupRestartExpiry is how long zulu's stored answer is valid: its
	// ttl-floor, because the stub answers it with TTL 0.
	domainGroupRestartExpiry = time.Second
)

// domainGroupRestartConfig is the daemon config both starts read: the two
// groups, and one table whose two terms name them.
func domainGroupRestartConfig() string {
	return `system {
	name-server 127.0.0.1;
	dns { resolv-conf-path ""; }
}
firewall {
	backend nft;
	domain-group ` + domainGroupRestartLong + ` {
		domain-names [ ` + domainGroupRestartLongName + ` ];
		ttl-floor 3600;
	}
	domain-group ` + domainGroupRestartShort + ` {
		domain-names [ ` + domainGroupRestartShortName + ` ];
		ttl-floor 1;
	}
	table ` + domainGroupRestartTable + ` {
		family inet;
		chain input {
			type filter;
			hook input;
			priority 0;
			policy accept;
			term allow-long {
				from { source-domain-group ` + domainGroupRestartLong + `; }
				then { accept; }
			}
			term allow-short {
				from { source-domain-group ` + domainGroupRestartShort + `; }
				then { accept; }
			}
		}
	}
}
`
}

// domainGroupQueryLog collects the stub's query log, one line per question.
// Safe for concurrent use: the stub writes from its serving goroutine while
// the driver reads.
type domainGroupQueryLog struct {
	mu    sync.Mutex
	lines bytes.Buffer
}

func (l *domainGroupQueryLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lines.Write(p)
}

func (l *domainGroupQueryLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lines.String()
}

// clear forgets every line logged so far, so the next read holds only the
// questions asked after it.
func (l *domainGroupQueryLog) clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines.Reset()
}

// domainGroupSetFor reads one group's IPv4 set out of the kernel.
func domainGroupSetFor(ctx context.Context, group string) (string, error) {
	return netfilterCommandOutput(ctx, "nft", "list", "set", "inet",
		domainGroupTablePrefix+domainGroupRestartTable, "domain_v4_"+group)
}

// domainGroupSetForHolds reports whether one group's IPv4 set holds address.
func domainGroupSetForHolds(ctx context.Context, group, address string) bool {
	out, err := domainGroupSetFor(ctx, group)
	if err != nil {
		return false
	}
	return strings.Contains(out, address)
}

// domainGroupRestartKeepsAnswers proves a daemon start over stored DNS answers
// asks upstream only for the names whose stored answer expired, and programs
// the others from the store.
//
// The question a unit test cannot decide: does the daemon's own start (the
// handshake, OnStarted, the store read, the schedule, the worker) honor the
// expiry the previous run wrote. It starts the daemon twice over one store:
//  1. Serves DNS, starts the daemon, and waits until both sets hold the first
//     answers, which is when both are stored.
//  2. Stops the daemon and waits until its table leaves the kernel, so the
//     second start's set cannot be the first run's left behind.
//  3. Waits until zulu's stored answer has expired, then changes both
//     answers and forgets the query log.
//  4. Starts the daemon again and waits until zulu's set holds its new
//     address: zulu was asked at start.
//  5. Checks alpha's set holds its stored address and not the new one, and
//     that the stub was not asked for alpha.
func domainGroupRestartKeepsAnswers(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("firewall-domain-group-restart-keeps-answers takes no arguments: %q", args)
	}
	workDir, err := filepath.Abs("restart-daemon")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(workDir, 0o750); err != nil {
		return err
	}
	configPath := filepath.Join(workDir, "restart.conf")
	if err := extra1Write(configPath, domainGroupRestartConfig()); err != nil {
		return err
	}
	// The store and the change log live in the config directory, so both starts
	// name the same one and the second reads what the first wrote. The store
	// selector is emptied too: the runner hands a .ci without option=storage
	// ze.test.storage=memory, and a store kept in the first daemon's memory dies
	// with it, so the second start would find nothing stored.
	environment := daemonOverrides(workDir)

	server, err := startDomainGroupStub()
	if err != nil {
		return err
	}
	defer closeDomainGroupStub(server)
	queries := &domainGroupQueryLog{}
	server.LogQueries(queries)
	domainGroupRestartAnswer(server, domainGroupRestartLongName, domainGroupRestartLongFirst, domainGroupRestartLongTTL)
	domainGroupRestartAnswer(server, domainGroupRestartShortName, domainGroupRestartShortFirst, 0)

	first, err := extra1StartDaemon(ctx, configPath, filepath.Join(workDir, "daemon1.log"), environment)
	if err != nil {
		return err
	}
	if !Poll(ctx, domainGroupRestartPollAttempts, netfilterPollDelay, func() bool {
		return domainGroupSetForHolds(ctx, domainGroupRestartLong, domainGroupRestartLongFirst) &&
			domainGroupSetForHolds(ctx, domainGroupRestartShort, domainGroupRestartShortFirst)
	}) {
		first.stop()
		return fmt.Errorf("the first start never programmed both groups; daemon log:\n%s", first.contents())
	}
	first.stop()
	stoppedAt := time.Now()

	if !Poll(ctx, domainGroupRestartPollAttempts, netfilterPollDelay, func() bool {
		_, readErr := domainGroupSetFor(ctx, domainGroupRestartLong)
		return readErr != nil
	}) {
		return errors.New("the stopped daemon left its table in the kernel, so the second start's set could not be told from the first's")
	}

	// zulu answered for the last time before the stop, so its stored expiry is
	// at most domainGroupRestartExpiry after stoppedAt. Waiting that out is the
	// behavior under test: the expiry is the zulu group's ttl-floor in the
	// config above.
	if remaining := time.Until(stoppedAt.Add(2 * domainGroupRestartExpiry)); remaining > 0 {
		if err := extra1Wait(ctx, remaining); err != nil {
			return err
		}
	}
	domainGroupRestartAnswer(server, domainGroupRestartLongName, domainGroupRestartLongSecond, domainGroupRestartLongTTL)
	domainGroupRestartAnswer(server, domainGroupRestartShortName, domainGroupRestartShortSecond, 0)
	queries.clear()

	second, err := extra1StartDaemon(ctx, configPath, filepath.Join(workDir, "daemon2.log"), environment)
	if err != nil {
		return err
	}
	defer second.stop()
	if !Poll(ctx, domainGroupRestartPollAttempts, netfilterPollDelay, func() bool {
		return domainGroupSetForHolds(ctx, domainGroupRestartShort, domainGroupRestartShortSecond)
	}) {
		return fmt.Errorf("the second start never asked for %s, whose stored answer had expired; daemon log:\n%s",
			domainGroupRestartShortName, second.contents())
	}

	longSet, err := domainGroupSetFor(ctx, domainGroupRestartLong)
	if err != nil {
		return fmt.Errorf("the second start did not program %s from its stored answer: %w", domainGroupRestartLong, err)
	}
	if !strings.Contains(longSet, domainGroupRestartLongFirst) {
		return fmt.Errorf("the second start did not program %s's stored address; the set reads:\n%s", domainGroupRestartLong, longSet)
	}
	if strings.Contains(longSet, domainGroupRestartLongSecond) {
		return fmt.Errorf("the second start asked again for %s, whose stored answer had not expired; the set reads:\n%s",
			domainGroupRestartLongName, longSet)
	}
	asked := queries.String()
	if strings.Contains(asked, domainGroupRestartLongName) {
		return fmt.Errorf("the second start queried %s, whose stored answer had not expired; queries:\n%s",
			domainGroupRestartLongName, asked)
	}
	if !strings.Contains(asked, domainGroupRestartShortName) {
		return fmt.Errorf("the query log does not show the %s query the set proves; queries:\n%s",
			domainGroupRestartShortName, asked)
	}

	// Printed only after every check above held, so the `.ci` markers cannot
	// appear on a failed run.
	var tb textbuf.Buffer
	tb.Str(longSet).Str("\nrestart-asked-for ").Str(domainGroupRestartShortName).
		Str(" not ").Str(domainGroupRestartLongName).Byte('\n')
	return tb.StdOut()
}

// domainGroupRestartAnswer makes the stub answer name with one IPv4 address
// and no IPv6 one, at ttl.
func domainGroupRestartAnswer(server *dnsmock.Server, name, address string, ttl uint32) {
	server.Set(name, mdns.TypeA, dnsmock.Answer{TTL: ttl, Addresses: []netip.Addr{netip.MustParseAddr(address)}})
	server.Set(name, mdns.TypeAAAA, dnsmock.Answer{TTL: ttl})
}
