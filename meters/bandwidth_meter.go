package meters

import (
	"sync"
	"time"
)

// BandwidthMeter counts the number of bytes written to it over time.
// Safe for concurrent use: multiple goroutines may Write while another
// goroutine reads Bandwidth/BytesRead/Duration.
type BandwidthMeter struct {
	mu        sync.Mutex
	bytesRead uint64
	start     time.Time
	lastRead  time.Time
}

// Write implements the io.Writer interface.
func (br *BandwidthMeter) Write(p []byte) (int, error) {
	// Always completes and never returns an error.
	br.mu.Lock()
	defer br.mu.Unlock()

	br.lastRead = time.Now().UTC()
	n := len(p)
	br.bytesRead += uint64(n)
	if br.start.IsZero() {
		br.start = br.lastRead
	}

	return n, nil
}

// Start records the start time
func (br *BandwidthMeter) Start() {
	br.mu.Lock()
	defer br.mu.Unlock()
	br.start = time.Now().UTC()
}

// Bandwidth returns the current bandwidth
func (br *BandwidthMeter) Bandwidth() (bytesPerSec float64) {
	br.mu.Lock()
	defer br.mu.Unlock()

	deltaSecs := br.lastRead.Sub(br.start).Seconds()
	if deltaSecs <= 0 {
		return 0
	}
	bytesPerSec = float64(br.bytesRead) / deltaSecs
	return
}

// BytesRead returns the number of bytes read by this BandwidthMeter
func (br *BandwidthMeter) BytesRead() (bytes uint64) {
	br.mu.Lock()
	defer br.mu.Unlock()
	bytes = br.bytesRead
	return
}

// Duration returns the current duration
func (br *BandwidthMeter) Duration() (duration time.Duration) {
	br.mu.Lock()
	defer br.mu.Unlock()
	duration = br.lastRead.Sub(br.start)
	return
}
