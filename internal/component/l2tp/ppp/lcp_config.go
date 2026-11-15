// Design: docs/research/l2tpv2-ze-integration.md -- immutable session negotiation settings
// Related: start_session.go -- transports hand a validated snapshot to the driver

package ppp

import (
	"fmt"
	"time"
)

// LCPConfig holds the validated, immutable negotiation settings of one session.
// Its zero value is invalid. Transports MUST construct it with NewLCPConfig
// before submitting StartSession; the driver MUST reject an invalid snapshot.
// Copies are safe for concurrent use.
type LCPConfig struct {
	restartTimer time.Duration
	maxTerminate int
	maxConfigure int
	maxFailure   int
}

// NewLCPConfig validates the bounds declared by the shared LCP YANG grouping.
// The caller supplies defaults from the schema; this constructor invents none.
func NewLCPConfig(restartTimer time.Duration, maxTerminate, maxConfigure, maxFailure int) (LCPConfig, error) {
	c := LCPConfig{restartTimer, maxTerminate, maxConfigure, maxFailure}
	if err := c.Validate(); err != nil {
		return LCPConfig{}, err
	}
	return c, nil
}

// Validate also rejects an uninitialized snapshot before the driver owns any I/O.
func (c LCPConfig) Validate() error {
	if c.restartTimer < 100*time.Millisecond {
		return fmt.Errorf("lcp restart-timer %s is outside 100..60000 milliseconds", c.restartTimer)
	}
	if c.restartTimer > 60*time.Second {
		return fmt.Errorf("lcp restart-timer %s is outside 100..60000 milliseconds", c.restartTimer)
	}
	if c.restartTimer%time.Millisecond != 0 {
		return fmt.Errorf("lcp restart-timer %s is not whole milliseconds", c.restartTimer)
	}
	for _, limit := range []struct {
		name  string
		value int
	}{
		{"max-terminate", c.maxTerminate}, {"max-configure", c.maxConfigure}, {"max-failure", c.maxFailure},
	} {
		if limit.value < 1 {
			return fmt.Errorf("lcp %s %d is outside 1..255", limit.name, limit.value)
		}
		if limit.value > 255 {
			return fmt.Errorf("lcp %s %d is outside 1..255", limit.name, limit.value)
		}
	}
	return nil
}

// RestartTimer is the wait after a Configure-Request, Terminate-Request or zrc.
func (c LCPConfig) RestartTimer() time.Duration { return c.restartTimer }

// MaxTerminate counts unanswered Terminate-Requests, including the first one.
func (c LCPConfig) MaxTerminate() int { return c.maxTerminate }

// MaxConfigure counts Configure-Requests without a valid negotiation reply.
func (c LCPConfig) MaxConfigure() int { return c.maxConfigure }

// MaxFailure counts Configure-Naks before unacceptable options are rejected.
func (c LCPConfig) MaxFailure() int { return c.maxFailure }
