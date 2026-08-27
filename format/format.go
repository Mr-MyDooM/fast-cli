package format

import "fmt"
import "github.com/dustin/go-humanize"

// BitsPerSec formats a byte count as a bits-per-second rate
func BitsPerSec(bytes float64) string {
	prettySize, prettyUnit := humanize.ComputeSI(bytes * 8)
	return fmt.Sprintf("%7.2f %sbps", prettySize, prettyUnit)
}

// BytesPerSec formats a byte count as a bytes-per-second rate
func BytesPerSec(bytes float64) string {
	prettySize, prettyUnit := humanize.ComputeSI(bytes)
	return fmt.Sprintf("%7.2f %sB/s", prettySize, prettyUnit)
}

// Bytes formats a byte count
func Bytes(bytes uint64) string {
	prettySize, prettyUnit := humanize.ComputeSI(float64(bytes))
	return fmt.Sprintf("%3.f %sB", prettySize, prettyUnit)
}

// Percent formats a percent
func Percent(current uint64, total uint64) string {
	return fmt.Sprintf("%5.1f%%", float64(current)/float64(total)*100)
}
