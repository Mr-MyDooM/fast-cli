package format

import "testing"

func TestBitsPerSec(t *testing.T) {
	got := BitsPerSec(125000)
	want := "   1.00 Mbps"
	if got != want {
		t.Errorf("BitsPerSec(125000) = %q, want %q", got, want)
	}
}

func TestBytesPerSec(t *testing.T) {
	got := BytesPerSec(1000)
	want := "   1.00 kB/s"
	if got != want {
		t.Errorf("BytesPerSec(1000) = %q, want %q", got, want)
	}
}

func TestBytes(t *testing.T) {
	got := Bytes(1000)
	want := "  1 kB"
	if got != want {
		t.Errorf("Bytes(1000) = %q, want %q", got, want)
	}
}

func TestSparklineEmpty(t *testing.T) {
	if got := Sparkline(nil); got != "" {
		t.Errorf("Sparkline(nil) = %q, want empty", got)
	}
}

func TestSparklineFlat(t *testing.T) {
	got := Sparkline([]float64{5, 5, 5})
	want := "▁▁▁"
	if got != want {
		t.Errorf("Sparkline(flat) = %q, want %q", got, want)
	}
}

func TestSparklineRange(t *testing.T) {
	got := Sparkline([]float64{0, 50, 100})
	want := "▁▄█"
	if got != want {
		t.Errorf("Sparkline(0,50,100) = %q, want %q", got, want)
	}
}

func TestPercent(t *testing.T) {
	got := Percent(50, 200)
	want := " 25.0%"
	if got != want {
		t.Errorf("Percent(50, 200) = %q, want %q", got, want)
	}
}
