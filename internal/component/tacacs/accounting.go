// Design: (none -- new TACACS+ component)
// Overview: client.go -- TACACS+ TCP client
// Related: authenticator.go -- auth bridge (sibling wrapper around client)

// TacacsAccountant implements aaa.Accountant for TACACS+ accounting.
// RFC 8907 Section 7. Sends START/STOP records for command execution.
package tacacs

import (
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// acctMsg is an accounting request queued for the background worker.
type acctMsg struct {
	req *AcctRequest
}

// Command IDs span accountant generations because reload can leave commands
// active on both the retired and the newly installed server configuration.
var accountingTaskSeq atomic.Uint64

// TacacsAccountant sends TACACS+ accounting records for command execution.
// It implements aaa.Accountant.
// Accounting failures are logged locally and never refuse command execution.
// A full queue applies backpressure rather than discarding commands.
//
// Safe for concurrent use. The owner MUST call Start before use and Stop
// before closing the client. Stop drains accepted records and waits for the
// worker; each exchange remains bounded by the client's server timeouts.
type TacacsAccountant struct {
	client    *TacacsClient
	logger    *slog.Logger
	drops     atomic.Uint64 // records refused after shutdown
	queue     chan acctMsg  // buffered channel; closed under mu by Stop
	done      chan struct{} // closed when worker exits
	mu        sync.RWMutex  // protects stopped and serializes enqueue with close
	stopped   bool
	startOnce sync.Once
	stopOnce  sync.Once
}

// NewTacacsAccountant creates a TacacsAccountant.
// MUST call Start() to launch the worker before sending records.
func NewTacacsAccountant(client *TacacsClient, logger *slog.Logger) *TacacsAccountant {
	if logger == nil {
		logger = slog.Default()
	}
	return &TacacsAccountant{
		client: client,
		logger: logger,
		queue:  make(chan acctMsg, 64),
		done:   make(chan struct{}),
	}
}

// Start launches the background worker once. The owner MUST call Stop before
// closing the client, including when no command has been submitted.
func (a *TacacsAccountant) Start() {
	a.startOnce.Do(func() {
		go a.worker()
	})
}

// Stop drains all accepted records and waits for the worker. The owner MUST
// call Stop before closing the client. Safe to call more than once and before
// Start; new records after shutdown are refused and counted.
func (a *TacacsAccountant) Stop() {
	a.stopOnce.Do(func() {
		a.Start()
		a.mu.Lock()
		a.stopped = true
		close(a.queue)
		a.mu.Unlock()
	})
	<-a.done
}

// DropCount returns the number of records refused after shutdown.
func (a *TacacsAccountant) DropCount() uint64 {
	if a == nil {
		return 0
	}
	return a.drops.Load()
}

// worker drains the queue until Stop closes it. RFC 8907 Section 8.3:
// "TACACS+ client devices MUST be configured to send an accounting start
// packet for every command entered, irrespective of how the commands were
// authorized." Shutdown must not discard records that were already accepted.
func (a *TacacsAccountant) worker() {
	defer close(a.done)
	for msg := range a.queue {
		a.processOne(msg)
	}
}

// processOne sends a single accounting request and logs the outcome.
func (a *TacacsAccountant) processOne(msg acctMsg) {
	reply, err := a.client.SendAccounting(msg.req)
	if err != nil {
		a.logger.Warn("TACACS+ accounting failed",
			"user", msg.req.User, "error", err)
		return
	}
	if reply.Status != AcctStatusSuccess {
		a.logger.Warn("TACACS+ accounting rejected",
			"user", msg.req.User, "status", reply.Status)
	}
}

// enqueue waits for a slot in the bounded queue. Holding mu through the send
// lets Stop close the queue only after all accepted senders have finished.
// RFC 8907 Section 8.3: "TACACS+ client devices MUST be configured to send an
// accounting start packet for every command entered, irrespective of how the
// commands were authorized." Queue saturation is not permission to omit one.
func (a *TacacsAccountant) enqueue(req *AcctRequest) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.stopped {
		return false
	}
	a.queue <- acctMsg{req: req}
	return true
}

// CommandStart sends an accounting START record. Returns a task ID for correlation.
// Waits for queue capacity; it never discards a command because the queue is full.
func (a *TacacsAccountant) CommandStart(username, remoteAddr, command string) string {
	return a.commandStart(username, remoteAddr, splitTacacsArgs(command))
}

// CommandStartArgs records the dispatcher's original argument boundaries.
func (a *TacacsAccountant) CommandStartArgs(username, remoteAddr string, tokens []string) string {
	return a.commandStart(username, remoteAddr, splitTacacsTokens(tokens))
}

func (a *TacacsAccountant) commandStart(username, remoteAddr string, commandArgs []string) string {
	taskID := strconv.FormatUint(accountingTaskSeq.Add(1), 10)

	// RFC 8907 Section 8.3: the accounting arguments "MUST precede any
	// argument-value pairs that are defined in 'Authorization' (Section 6)",
	// so task_id and start_time come before service, cmd and cmd-arg.
	// start_time is epoch seconds: Section 8.1 says "The time zone MUST be
	// UTC unless a time zone argument is specified", and Unix() is UTC by
	// definition whatever time.Local holds.
	args := accountingArguments(taskID, textbuf.StrInt("start_time=", time.Now().Unix()), commandArgs)
	req := &AcctRequest{
		Flags:         AcctFlagStart,
		AuthenMethod:  0x06, // TACACS+
		PrivLvl:       1,    // default user
		AuthenType:    0x01, // ASCII
		AuthenService: 0x01, // login
		User:          username,
		Port:          portSSH,
		RemAddr:       remoteAddr,
		Args:          args,
	}

	if !a.enqueue(req) {
		a.drops.Add(1)
		a.logger.Warn("TACACS+ accounting stopped, refusing START", "username", username)
	}

	return taskID
}

// CommandStop sends an accounting STOP record.
// Waits for queue capacity; it never discards a record because the queue is full.
func (a *TacacsAccountant) CommandStop(taskID, username, remoteAddr, command string) {
	a.commandStop(taskID, username, remoteAddr, splitTacacsArgs(command))
}

// CommandStopArgs records the same argument boundaries as CommandStartArgs.
func (a *TacacsAccountant) CommandStopArgs(taskID, username, remoteAddr string, tokens []string) {
	a.commandStop(taskID, username, remoteAddr, splitTacacsTokens(tokens))
}

func (a *TacacsAccountant) commandStop(taskID, username, remoteAddr string, commandArgs []string) {
	// RFC 8907 Section 8.3: accounting arguments precede the Section 6 ones
	// (see CommandStart), and Section 7.2 says "The STOP flag MUST NOT be
	// set in conjunction with the WATCHDOG flag": the record carries
	// AcctFlagStop alone.
	stopArgs := accountingArguments(taskID, textbuf.StrInt("stop_time=", time.Now().Unix()), commandArgs)
	req := &AcctRequest{
		Flags:         AcctFlagStop,
		AuthenMethod:  0x06, // TACACS+
		PrivLvl:       1,
		AuthenType:    0x01,
		AuthenService: 0x01,
		User:          username,
		Port:          portSSH,
		RemAddr:       remoteAddr,
		Args:          stopArgs,
	}

	if !a.enqueue(req) {
		a.drops.Add(1)
		a.logger.Warn("TACACS+ accounting stopped, refusing STOP", "username", username)
	}
}
