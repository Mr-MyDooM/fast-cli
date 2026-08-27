package main

import "os"
import "fmt"
import "io"
import "encoding/json"
import "net/http"
import "strconv"
import "time"

import "github.com/spf13/cobra"
import "github.com/gesquive/cli"
import "github.com/gesquive/fast-cli/fast"
import "github.com/gesquive/fast-cli/format"
import "github.com/gesquive/fast-cli/meters"

var version = "v0.2.10"
var dirty = ""
var displayVersion string

var cfgFile string
var logDebug bool
var notHTTPS bool
var simpleProgress bool
var showVersion bool
var dlCount uint64
var showBytes bool
var jsonOutput bool

// RootCmd is the only command
var RootCmd = &cobra.Command{
	Use:   "fast-cli",
	Short: "Estimates your current internet download speed",
	Long:  `fast-cli estimates your current internet download speed by performing a series of downloads from Netflix's fast.com servers.`,
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
	RootCmd.SetHelpTemplate(fmt.Sprintf("%s\nVersion:\n  github.com/gesquive/%s\n",
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
	RootCmd.PersistentFlags().Uint64VarP(&dlCount, "count", "c", 3, "Number of parallel connections to use")
	RootCmd.PersistentFlags().BoolVarP(&showBytes, "bytes", "b", false, "Display speed in bytes per second instead of bits per second")
	RootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output the result as JSON")

	RootCmd.PersistentFlags().MarkHidden("debug")
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

func run(cmd *cobra.Command, args []string) {
	if showVersion {
		cli.Infoln(displayVersion)
		os.Exit(0)
	}
	fast.UseHTTPS = !notHTTPS
	urls, err := fast.GetDlUrls(dlCount)
	if err != nil {
		cli.Warnf("Could not get urls from fast.com: %v\n", err)
	}
	cli.Debugf("Got %d from fast service\n", len(urls))

	if len(urls) == 0 {
		cli.Warnf("Using fallback endpoint\n")
		urls = append(urls, fast.GetDefaultURL())
	}

	err = calculateBandwidth(urls)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// Result is the outcome of a bandwidth test, used for JSON output.
type Result struct {
	BitsPerSec  float64 `json:"bits_per_sec"`
	BytesPerSec float64 `json:"bytes_per_sec"`
	BytesRead   uint64  `json:"bytes_read"`
	Seconds     float64 `json:"seconds"`
	LatencyMs   float64 `json:"latency_ms"`
}

func formatSpeed(bytesPerSec float64) string {
	if showBytes {
		return format.BytesPerSec(bytesPerSec)
	}
	return format.BitsPerSec(bytesPerSec)
}

func calculateBandwidth(urls []string) (err error) {
	client := &http.Client{}
	count := uint64(len(urls))

	primaryBandwidthReader := meters.BandwidthMeter{}
	bandwidthMeter := meters.BandwidthMeter{}
	ch := make(chan *copyResults, 1)
	bytesToRead := uint64(0)
	completed := uint64(0)
	var latency time.Duration

	for i := uint64(0); i < count; i++ {
		// Create the HTTP request
		request, err := http.NewRequest("GET", urls[i], nil)
		if err != nil {
			return err
		}
		request.Header.Set("User-Agent", displayVersion)

		// Get the HTTP Response, timing time-to-first-byte on the leading connection
		requestStart := time.Now()
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()

		// Set information for the leading index
		if i == 0 {
			latency = time.Since(requestStart)
			cli.Debugf("Latency=%s\n", latency)

			// Try to get content length
			contentLength := response.Header.Get("Content-Length")
			calculatedLength, err := strconv.Atoi(contentLength)
			if err != nil {
				calculatedLength = 26214400
			}
			bytesToRead = uint64(calculatedLength)
			cli.Debugf("Download Size=%d\n", bytesToRead)

			tapMeter := io.TeeReader(response.Body, &primaryBandwidthReader)
			go asyncCopy(i, ch, &bandwidthMeter, tapMeter)
		} else {
			// Start reading
			go asyncCopy(i, ch, &bandwidthMeter, response.Body)
		}

	}

	showProgress := !simpleProgress && !jsonOutput
	if showProgress {
		cli.Infof("Estimating current download speed\n")
	}
	for {
		select {
		case results := <-ch:
			if results.err != nil {
				fmt.Fprintf(os.Stdout, "\n%v\n", results.err)
				os.Exit(1)
			}

			completed++
			bandwidth := bandwidthMeter.Bandwidth()
			switch {
			case jsonOutput:
				result := Result{
					BitsPerSec:  bandwidth * 8,
					BytesPerSec: bandwidth,
					BytesRead:   bandwidthMeter.BytesRead(),
					Seconds:     bandwidthMeter.Duration().Seconds(),
					LatencyMs:   float64(latency.Microseconds()) / 1000,
				}
				encoded, err := json.Marshal(result)
				if err != nil {
					return err
				}
				fmt.Println(string(encoded))
			case showProgress:
				fmt.Printf("\r%s - %s",
					formatSpeed(bandwidth),
					format.Percent(primaryBandwidthReader.BytesRead(), bytesToRead))
				fmt.Printf("  \n")
				fmt.Printf("Latency: %.1f ms\n", float64(latency.Microseconds())/1000)
				fmt.Printf("Completed in %.1f seconds\n", bandwidthMeter.Duration().Seconds())
			default:
				fmt.Printf("%s\n", formatSpeed(bandwidth))
			}
			return nil
		case <-time.After(100 * time.Millisecond):
			if showProgress {
				fmt.Printf("\r%s - %s",
					formatSpeed(bandwidthMeter.Bandwidth()),
					format.Percent(primaryBandwidthReader.BytesRead(), bytesToRead))
			}
		}
	}
}

type copyResults struct {
	index        uint64
	bytesWritten uint64
	err          error
}

func asyncCopy(index uint64, channel chan *copyResults, writer io.Writer, reader io.Reader) {
	bytesWritten, err := io.Copy(writer, reader)
	channel <- &copyResults{index, uint64(bytesWritten), err}
}
