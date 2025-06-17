package report

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"time"
)

// Reporter tracks overall progress and prints it once a second.
type Reporter struct {
	total   int64
	current int64
	ticker  *time.Ticker
	done    chan struct{}
}

// New creates and starts a reporter for total work units.
func New(total int) *Reporter {
	r := &Reporter{
		total:  int64(total),
		ticker: time.NewTicker(time.Second),
		done:   make(chan struct{}),
	}
	go r.loop()
	return r
}

// Inc adds n completed units (thread-safe).
func (r *Reporter) Inc(n int64) { atomic.AddInt64(&r.current, n) }

// Close stops the reporter and prints a final newline.
func (r *Reporter) Close() {
	close(r.done)
	r.print()         // final state
	fmt.Print("\n\n") // move to fresh line
}

func (r *Reporter) loop() {
	for {
		select {
		case <-r.ticker.C:
			r.print()
		case <-r.done:
			r.ticker.Stop()
			return
		}
	}
}

func (r *Reporter) print() {
	cur := atomic.LoadInt64(&r.current)
	pct := float64(cur) / float64(r.total) * 100

	var m runtime.MemStats
	runtime.ReadMemStats(&m)          // heap allocations only
	heapMiB := float64(m.Alloc) / 1e6 // ≈MiB, good enough for CLI

	// “\r” rewinds the line so we overwrite the previous output
	fmt.Printf("\r[PROGRESS] %d / %d (%.1f%%) | RAM: %.1f MiB",
		cur, r.total, pct, heapMiB)
}
