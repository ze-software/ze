package progress

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gokrazy/internal/humanize"
)

var bytesTransferred uint64

func Reset() uint64 {
	return atomic.SwapUint64(&bytesTransferred, 0)
}

type Writer struct{}

func (w Writer) Write(p []byte) (n int, err error) {
	atomic.AddUint64(&bytesTransferred, uint64(len(p)))
	return len(p), nil
}

type Reporter struct {
	total uint64

	mu     sync.Mutex
	status string
}

func (p *Reporter) SetStatus(status string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status = status
}

func (p *Reporter) SetTotal(total uint64) {
	atomic.StoreUint64(&p.total, total)
}

func (p *Reporter) getStatus() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

func (p *Reporter) Report(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	last := atomic.LoadUint64(&bytesTransferred)
	for {
		select {
		case <-ticker.C:
			transferred := atomic.LoadUint64(&bytesTransferred)
			if transferred < last {
				// transferred was reset
				last = 0
			}
			bytesPerS := transferred - last
			last = transferred
			rate := humanize.BPS(bytesPerS)
			status := rate
			if total := atomic.LoadUint64(&p.total); total > 0 {
				pct := float64(transferred) / float64(total) * 100
				status = fmt.Sprintf("%02.2f%% of %s, uploading at %s",
					pct,
					humanize.Bytes(total),
					rate)
			}
			fmt.Printf("\r[%s] %s                 ", p.getStatus(), status)
		case <-ctx.Done():
			return
		}
	}
}
