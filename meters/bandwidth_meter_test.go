package meters

import (
	"testing"
	"time"
)

func TestWriteTracksBytesRead(t *testing.T) {
	var m BandwidthMeter
	n, err := m.Write(make([]byte, 100))
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if n != 100 {
		t.Errorf("Write returned n = %d, want 100", n)
	}
	if m.BytesRead() != 100 {
		t.Errorf("BytesRead() = %d, want 100", m.BytesRead())
	}
}

func TestBandwidthComputesRate(t *testing.T) {
	var m BandwidthMeter
	m.start = time.Now().UTC()
	m.Write(make([]byte, 100))
	m.lastRead = m.start.Add(1 * time.Second)

	got := m.Bandwidth()
	want := 100.0
	if got != want {
		t.Errorf("Bandwidth() = %f, want %f", got, want)
	}
}

func TestBandwidthZeroDurationReturnsZero(t *testing.T) {
	var m BandwidthMeter
	now := time.Now().UTC()
	m.start = now
	m.lastRead = now
	m.Write(make([]byte, 100))
	m.lastRead = now

	if got := m.Bandwidth(); got != 0 {
		t.Errorf("Bandwidth() with zero duration = %f, want 0", got)
	}
}

func TestDuration(t *testing.T) {
	var m BandwidthMeter
	m.start = time.Now().UTC()
	m.lastRead = m.start.Add(2 * time.Second)

	if m.Duration() != 2*time.Second {
		t.Errorf("Duration() = %v, want 2s", m.Duration())
	}
}
