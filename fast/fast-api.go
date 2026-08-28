package fast

import "fmt"
import "bytes"
import "encoding/json"
import "net/http"
import "net/url"
import "io"
import "regexp"
import "strings"
import "time"
import "github.com/gesquive/cli"

// UseHTTPS sets if HTTPS is used
var UseHTTPS = true

// httpClient is used for all scraping/API calls (not the bandwidth test
// downloads themselves, which need their own long-lived client).
var httpClient = &http.Client{Timeout: 15 * time.Second}

// maxScrapeBodyBytes bounds how much we'll read from fast.com's HTML/JS/API
// responses, which are all small, known-shape pages/JSON, not test downloads.
const maxScrapeBodyBytes = 5 * 1024 * 1024

// allowedDomains restricts which hosts we'll ever issue a GET to when following
// data scraped out of fast.com's pages, so a compromised/spoofed response
// can't be used to make this process request arbitrary internal or external
// hosts (SSRF).
var allowedDomains = []string{"fast.com", "nflxvideo.net", "netflix.com"}

func isAllowedHost(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	for _, domain := range allowedDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

// Client describes the network fast.com sees this test running from.
type Client struct {
	IP      string
	ISP     string
	City    string
	Country string
}

// Target is a single download endpoint fast.com assigned for this test.
type Target struct {
	URL     string
	City    string
	Country string
}

type location struct {
	City    string `json:"city"`
	Country string `json:"country"`
}

type speedtestConfig struct {
	Client struct {
		IP       string   `json:"ip"`
		ISP      string   `json:"isp"`
		Location location `json:"location"`
	} `json:"client"`
	Targets []struct {
		URL      string   `json:"url"`
		Location location `json:"location"`
	} `json:"targets"`
}

// GetSpeedtestConfig fetches the client's network info and a list of
// download targets from fast.com.
func GetSpeedtestConfig(urlCount uint64) (client Client, targets []Target, err error) {
	token, err := getFastToken()
	if err != nil {
		return client, nil, err
	}

	httpProtocol := "https"
	if !UseHTTPS {
		httpProtocol = "http"
	}

	apiURL := fmt.Sprintf("%s://api.fast.com/netflix/speedtest/v2?https=%t&token=%s&urlCount=%d",
		httpProtocol, UseHTTPS, token, urlCount)
	cli.Debug("getting speedtest config from %s", apiURL)

	jsonData, err := getPage(apiURL)
	if err != nil {
		return client, nil, fmt.Errorf("failed to fetch fast.com speedtest config: %w", err)
	}

	var config speedtestConfig
	if err := json.Unmarshal([]byte(jsonData), &config); err != nil {
		return client, nil, fmt.Errorf("failed to parse fast.com speedtest config: %w", err)
	}

	client = Client{
		IP:      config.Client.IP,
		ISP:     config.Client.ISP,
		City:    config.Client.Location.City,
		Country: config.Client.Location.Country,
	}

	cli.Debug("targets:")
	for _, t := range config.Targets {
		if !isAllowedHost(t.URL) {
			cli.Warn("ignoring download url with unexpected host: %s", t.URL)
			continue
		}
		targets = append(targets, Target{
			URL:     t.URL,
			City:    t.Location.City,
			Country: t.Location.Country,
		})
		cli.Debug(" - %s (%s, %s)", t.URL, t.Location.City, t.Location.Country)
	}

	return client, targets, nil
}

// GetDefaultURL returns the fallback download URL
func GetDefaultURL() (url string) {
	httpProtocol := "https"
	if !UseHTTPS {
		httpProtocol = "http"
	}
	url = fmt.Sprintf("%s://api.fast.com/netflix/speedtest", httpProtocol)
	return
}

func getFastToken() (token string, err error) {
	baseURL := "https://fast.com"
	if !UseHTTPS {
		baseURL = "http://fast.com"
	}
	fastBody, err := getPage(baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch %s: %w", baseURL, err)
	}

	// Extract the app script url
	re := regexp.MustCompile(`app-[A-Za-z0-9]+\.js`)
	scriptNames := re.FindAllString(fastBody, 1)
	if len(scriptNames) == 0 {
		return "", fmt.Errorf("could not find fast.com app script, page layout may have changed")
	}

	scriptURL := fmt.Sprintf("%s/%s", baseURL, scriptNames[0])
	if !isAllowedHost(scriptURL) {
		return "", fmt.Errorf("refusing to fetch script from unexpected host: %s", scriptURL)
	}
	cli.Debug("trying to get fast api token from %s", scriptURL)

	// Extract the token
	scriptBody, err := getPage(scriptURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch %s: %w", scriptURL, err)
	}

	re = regexp.MustCompile("token:\"[[:alpha:]]*\"")
	tokens := re.FindAllString(scriptBody, 1)

	if len(tokens) == 0 {
		return "", fmt.Errorf("no token found, fast.com script format may have changed")
	}

	token = tokens[0][7 : len(tokens[0])-1]
	cli.Debug("token found: %s", token)

	return token, nil
}

func getPage(pageURL string) (contents string, err error) {
	// Create the string buffer
	buffer := bytes.NewBuffer(nil)

	// Get the data
	resp, err := httpClient.Get(pageURL)
	if err != nil {
		return contents, err
	}
	defer resp.Body.Close()

	// Writer the body to file, capped so a hostile/misbehaving endpoint
	// can't exhaust memory on what should be a small html/js/json response.
	_, err = io.Copy(buffer, io.LimitReader(resp.Body, maxScrapeBodyBytes))
	if err != nil {
		return contents, err
	}
	contents = buffer.String()

	return
}
