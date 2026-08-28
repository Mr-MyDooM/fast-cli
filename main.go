package main

import "os"
import "fmt"
import "io"
import "bytes"
import "context"
import "encoding/json"
import "math/rand/v2"
import "net/http"
import "sync"
import "time"

import "github.com/spf13/cobra"
import "github.com/gesquive/cli"
import "github.com/Mr-MyDooM/fast-cli/fast"
import "github.com/Mr-MyDooM/fast-cli/format"
import "github.com/Mr-MyDooM/fast-cli/meters"

var version = "v0.2.10"
var dirty = ""
var displayVersion string

var cfgFile string
var logDebug bool
var notHTTPS bool
var simpleProgress bool
var showVersion bool
var dlCount uint64
var testDuration int
var showBytes bool
var jsonOutput bool
var noUpload bool

// RootCmd is the only command
var RootCmd = &cobra.Command{
	Use:   "fast-cli",
	Short: "Estimates your current internet download speed",
	Long:  `fast-cli estimates your current internet download and upload speed by performing a series of transfers to/from Netflix's fast.com servers.`,
	Run:   run,
}

func main() {
	displayVersion = fmt.Sprintf("fast-cli %s%s",
		version,
		dirty)
	Execute(displayVersion)
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(version string) {
	displayVersion = version
	RootCmd.SetHelpTemplate(fmt.Sprintf("%s\nVersion:\n  github.com/Mr-MyDooM/%s\n",
		RootCmd.HelpTemplate(), displayVersion))
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initLog)

	RootCmd.PersistentFlags().BoolVarP(&notHTTPS, "no-https", "n", false, "Do not use HTTPS when connecting")
	RootCmd.PersistentFlags().BoolVarP(&simpleProgress, "simple", "s", false, "Only display the result, no dynamic progress bar")
	RootCmd.PersistentFlags().BoolVar(&showVersion, "version", false, "Display the version number and exit")
	RootCmd.PersistentFlags().BoolVarP(&logDebug, "debug", "D", false, "Write debug messages to console")
	RootCmd.PersistentFlags().Uint64VarP(&dlCount, "count", "c", 3, fmt.Sprintf("Number of parallel connections to use (%d-%d)", minDlCount, maxDlCount))
	RootCmd.PersistentFlags().IntVarP(&testDuration, "duration", "d", 10, fmt.Sprintf("Test duration in seconds, per direction (%d-%d)", minDuration, maxDuration))
	RootCmd.PersistentFlags().BoolVarP(&showBytes, "bytes", "b", false, "Display speed in bytes per second instead of bits per second")
	RootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output the result as JSON")
	RootCmd.PersistentFlags().BoolVar(&noUpload, "no-upload", false, "Skip the upload speed test")

	if err := RootCmd.PersistentFlags().MarkHidden("debug"); err != nil {
		panic(err)
	}
}

func initLog() {
	cli.SetPrintLevel(cli.LevelInfo)
	if logDebug {
		cli.SetPrintLevel(cli.LevelDebug)
	}
	if notHTTPS {
		cli.Debugln("Not using HTTPS")
	} else {
		cli.Debugln("Using HTTPS")
	}
}

const minDlCount = 1
const maxDlCount = 8
const minDuration = 5
const maxDuration = 30
const sparklineWidth = 40

// uploadPayloadSize matches the size fast.com's own client uploads per
// request (MAX_PAYLOAD_BYTES in its app bundle).
const uploadPayloadSize = 25 * 1024 * 1024

func pushSample(samples []float64, v float64) []float64 {
	samples = append(samples, v)
	if len(samples) > sparklineWidth {
		samples = samples[len(samples)-sparklineWidth:]
	}
	return samples
}

func run(cmd *cobra.Command, args []string) {
	if showVersion {
		cli.Infoln(displayVersion)
		os.Exit(0)
	}
	if dlCount < minDlCount || dlCount > maxDlCount {
		fmt.Fprintf(os.Stderr, "count must be between %d and %d\n", minDlCount, maxDlCount)
		os.Exit(1)
	}
	if testDuration < minDuration || testDuration > maxDuration {
		fmt.Fprintf(os.Stderr, "duration must be between %d and %d seconds\n", minDuration, maxDuration)
		os.Exit(1)
	}
	fast.UseHTTPS = !notHTTPS
	client, targets, err := fast.GetSpeedtestConfig(dlCount)
	if err != nil {
		cli.Warnf("Could not get config from fast.com: %v\n", err)
	}
	cli.Debugf("Got %d targets from fast service\n", len(targets))

	urls := make([]string, 0, len(targets))
	for _, t := range targets {
		urls = append(urls, t.URL)
	}
	if len(urls) == 0 {
		cli.Warnf("Using fallback endpoint\n")
		urls = append(urls, fast.GetDefaultURL())
	}

	showProgress := !simpleProgress && !jsonOutput
	if showProgress && client.IP != "" {
		cli.Infof("Client      %s, %s   %s   %s\n", client.City, client.Country, client.IP, client.ISP)
	}
	if showProgress && len(targets) > 0 {
		cli.Infof("Server(s)   %s\n\n", serverSummary(targets))
	}

	if showProgress {
		cli.Infof("Estimating current download speed\n")
	}
	download, err := runTransfer(urls, showProgress, downloadWorker, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	if showProgress {
		fmt.Printf("\r%s  %5.1f%%  %s\n\n", formatSpeed(download.bandwidth), 100.0, format.Sparkline(download.samples))
		fmt.Printf("  Latency\n")
		fmt.Printf("    %-9s %.1f ms\n", "Unloaded", msFromDuration(download.unloadedLatency))
		fmt.Printf("    %-9s %.1f ms\n", "Loaded", msFromDuration(download.loadedLatency))
		fmt.Printf("  %-11s %.1f s\n\n", "Duration", download.duration.Seconds())
	}

	var upload transferOutcome
	if !noUpload {
		if showProgress {
			cli.Infof("Estimating current upload speed\n")
		}
		upload, err = runTransfer(urls, showProgress, uploadWorker, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		if showProgress {
			fmt.Printf("\r%s  %5.1f%%  %s\n\n", formatSpeed(upload.bandwidth), 100.0, format.Sparkline(upload.samples))
			fmt.Printf("  %-11s %.1f s\n", "Duration", upload.duration.Seconds())
		}
	}

	switch {
	case jsonOutput:
		printJSONResult(download, upload)
	case !showProgress:
		fmt.Printf("%s\n", formatSpeed(download.bandwidth))
		if !noUpload {
			fmt.Printf("%s\n", formatSpeed(upload.bandwidth))
		}
	}
}

// serverSummary lists the distinct city/country pairs among the assigned
// download targets.
func serverSummary(targets []fast.Target) string {
	seen := map[string]bool{}
	var places []string
	for _, t := range targets {
		place := fmt.Sprintf("%s, %s", t.City, t.Country)
		if t.City == "" || seen[place] {
			continue
		}
		seen[place] = true
		places = append(places, place)
	}
	if len(places) == 0 {
		return "unknown"
	}
	result := places[0]
	for _, p := range places[1:] {
		result += "; " + p
	}
	return result
}

// Result is the outcome of a speed test, used for JSON output.
type Result struct {
	DownloadBitsPerSec  float64 `json:"download_bits_per_sec"`
	DownloadBytesPerSec float64 `json:"download_bytes_per_sec"`
	DownloadBytes       uint64  `json:"download_bytes"`
	DownloadSeconds     float64 `json:"download_seconds"`
	UnloadedLatencyMs   float64 `json:"unloaded_latency_ms"`
	LoadedLatencyMs     float64 `json:"loaded_latency_ms"`
	UploadBitsPerSec    float64 `json:"upload_bits_per_sec,omitempty"`
	UploadBytesPerSec   float64 `json:"upload_bytes_per_sec,omitempty"`
	UploadBytes         uint64  `json:"upload_bytes,omitempty"`
	UploadSeconds       float64 `json:"upload_seconds,omitempty"`
}

func printJSONResult(download, upload transferOutcome) {
	result := Result{
		DownloadBitsPerSec:  download.bandwidth * 8,
		DownloadBytesPerSec: download.bandwidth,
		DownloadBytes:       download.bytesTotal,
		DownloadSeconds:     download.duration.Seconds(),
		UnloadedLatencyMs:   msFromDuration(download.unloadedLatency),
		LoadedLatencyMs:     msFromDuration(download.loadedLatency),
	}
	if !noUpload {
		result.UploadBitsPerSec = upload.bandwidth * 8
		result.UploadBytesPerSec = upload.bandwidth
		result.UploadBytes = upload.bytesTotal
		result.UploadSeconds = upload.duration.Seconds()
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(encoded))
}

func msFromDuration(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000
}

func formatSpeed(bytesPerSec float64) string {
	if showBytes {
		return format.BytesPerSec(bytesPerSec)
	}
	return format.BitsPerSec(bytesPerSec)
}

// latencyTracker records unloaded (pre-transfer) and loaded (measured while
// the link is saturated) latency samples.
type latencyTracker struct {
	mu       sync.Mutex
	unloaded time.Duration
	loaded   []time.Duration
}

func (lt *latencyTracker) setUnloaded(d time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	lt.unloaded = d
}

func (lt *latencyTracker) addLoaded(d time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	lt.loaded = append(lt.loaded, d)
}

func (lt *latencyTracker) snapshot() (unloaded time.Duration, loadedAvg time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	unloaded = lt.unloaded
	if len(lt.loaded) == 0 {
		return unloaded, 0
	}
	var total time.Duration
	for _, d := range lt.loaded {
		total += d
	}
	return unloaded, total / time.Duration(len(lt.loaded))
}

// transferWorker performs one direction's transfer against url, repeating
// until ctx is canceled, feeding bytes moved into meter.
type transferWorker func(ctx context.Context, client *http.Client, url string, meter io.Writer, isPrimary bool, latency *latencyTracker)

// downloadWorker repeatedly re-requests url until ctx is canceled, so the
// connection keeps flowing bytes for the full test duration instead of
// going idle once a single response body is exhausted.
func downloadWorker(ctx context.Context, client *http.Client, url string, meter io.Writer, isPrimary bool, latency *latencyTracker) {
	first := true
	for ctx.Err() == nil {
		request, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return
		}
		request.Header.Set("User-Agent", displayVersion)

		start := time.Now()
		response, err := client.Do(request)
		if err != nil {
			return
		}

		if isPrimary && first {
			latency.setUnloaded(time.Since(start))
			first = false
		}

		if _, err := io.Copy(meter, response.Body); err != nil {
			cli.Debugf("download stopped: %v\n", err)
		}
		if err := response.Body.Close(); err != nil {
			cli.Debugf("failed to close response body: %v\n", err)
		}
	}
}

var (
	uploadPayloadOnce sync.Once
	uploadPayload     []byte
)

// getUploadPayload lazily generates the random byte buffer POSTed on each
// upload request, matching the size fast.com's own client uploads.
func getUploadPayload() []byte {
	uploadPayloadOnce.Do(func() {
		uploadPayload = make([]byte, uploadPayloadSize)
		for i := range uploadPayload {
			uploadPayload[i] = byte(rand.IntN(256)) //#nosec G404 G115 -- bandwidth-test filler bytes, not security-sensitive; IntN(256) is always in [0,256)
		}
	})
	return uploadPayload
}

// countingReader forwards bytes read from r into w as they're consumed, so
// the bandwidth meter reflects bytes actually sent over the wire, not just
// the bytes queued for upload.
type countingReader struct {
	r io.Reader
	w io.Writer
}

func (cr *countingReader) Read(p []byte) (int, error) {
	n, err := cr.r.Read(p)
	if n > 0 {
		if _, werr := cr.w.Write(p[:n]); werr != nil {
			cli.Debugf("failed to record upload progress: %v\n", werr)
		}
	}
	return n, err
}

// uploadWorker repeatedly POSTs a fixed-size random payload to url until ctx
// is canceled, mirroring fast.com's own upload test (POST,
// application/octet-stream, ~25MB body per request).
func uploadWorker(ctx context.Context, client *http.Client, url string, meter io.Writer, isPrimary bool, latency *latencyTracker) {
	payload := getUploadPayload()
	for ctx.Err() == nil {
		body := &countingReader{r: bytes.NewReader(payload), w: meter}
		request, err := http.NewRequestWithContext(ctx, "POST", url, body)
		if err != nil {
			return
		}
		request.Header.Set("Content-Type", "application/octet-stream")
		request.Header.Set("User-Agent", displayVersion)
		request.ContentLength = int64(len(payload))

		response, err := client.Do(request)
		if err != nil {
			return
		}
		if _, err := io.Copy(io.Discard, response.Body); err != nil {
			cli.Debugf("upload response read failed: %v\n", err)
		}
		if err := response.Body.Close(); err != nil {
			cli.Debugf("failed to close upload response body: %v\n", err)
		}
	}
}

// loadedLatencyProbe periodically issues a tiny ranged request against url
// while the main transfers are in flight, to measure latency under load.
func loadedLatencyProbe(ctx context.Context, client *http.Client, url string, latency *latencyTracker) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			request, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				continue
			}
			request.Header.Set("Range", "bytes=0-0")
			request.Header.Set("User-Agent", displayVersion)

			start := time.Now()
			response, err := client.Do(request)
			if err != nil {
				continue
			}
			if _, err := io.Copy(io.Discard, response.Body); err != nil {
				cli.Debugf("latency probe read failed: %v\n", err)
			}
			if err := response.Body.Close(); err != nil {
				cli.Debugf("failed to close latency probe body: %v\n", err)
			}
			latency.addLoaded(time.Since(start))
		}
	}
}

// transferOutcome is the measured result of one direction's transfer.
type transferOutcome struct {
	bandwidth       float64
	bytesTotal      uint64
	duration        time.Duration
	unloadedLatency time.Duration
	loadedLatency   time.Duration
	samples         []float64
}

// runTransfer drives dlCount parallel workers for testDuration seconds,
// optionally showing a live progress line, and returns the measured
// bandwidth. When measureLatency is true, an unloaded-latency reading is
// taken off the first connection and a loaded-latency prober runs alongside
// the transfer.
func runTransfer(urls []string, showProgress bool, worker transferWorker, measureLatency bool) (transferOutcome, error) {
	client := &http.Client{}
	testStart := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(testDuration)*time.Second)
	defer cancel()

	meter := meters.BandwidthMeter{}
	latency := &latencyTracker{}

	var wg sync.WaitGroup
	for i, u := range urls {
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			worker(ctx, client, u, &meter, i == 0, latency)
		}(i, u)
	}
	if measureLatency {
		go loadedLatencyProbe(ctx, client, urls[0], latency)
	}

	var samples []float64
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

progressLoop:
	for {
		select {
		case <-ctx.Done():
			break progressLoop
		case <-ticker.C:
			bandwidth := meter.Bandwidth()
			samples = pushSample(samples, bandwidth)
			if showProgress {
				elapsed := time.Since(testStart)
				percent := elapsed.Seconds() / float64(testDuration) * 100
				if percent > 100 {
					percent = 100
				}
				fmt.Printf("\r%s  %5.1f%%  %s",
					formatSpeed(bandwidth),
					percent,
					format.Sparkline(samples))
			}
		}
	}
	wg.Wait()

	unloaded, loaded := latency.snapshot()
	return transferOutcome{
		bandwidth:       meter.Bandwidth(),
		bytesTotal:      meter.BytesRead(),
		duration:        meter.Duration(),
		unloadedLatency: unloaded,
		loadedLatency:   loaded,
		samples:         samples,
	}, nil
}
