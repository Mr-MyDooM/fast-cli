package fast

import "fmt"
import "bytes"
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

// allowedHosts restricts which hosts we'll ever issue a GET to when following
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

// GetDlUrls returns a list of urls to the fast api downloads
func GetDlUrls(urlCount uint64) (urls []string, err error) {
	token, err := getFastToken()
	if err != nil {
		return nil, err
	}

	httpProtocol := "https"
	if !UseHTTPS {
		httpProtocol = "http"
	}

	url := fmt.Sprintf("%s://api.fast.com/netflix/speedtest?https=%t&token=%s&urlCount=%d",
		httpProtocol, UseHTTPS, token, urlCount)
	cli.Debug("getting url list from %s", url)

	jsonData, err := getPage(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch fast.com url list: %w", err)
	}

	re := regexp.MustCompile("(?U)\"url\":\"(.*)\"")
	reUrls := re.FindAllStringSubmatch(jsonData, -1)

	cli.Debug("urls:")
	for _, arr := range reUrls {
		if !isAllowedHost(arr[1]) {
			cli.Warn("ignoring download url with unexpected host: %s", arr[1])
			continue
		}
		urls = append(urls, arr[1])
		cli.Debug(" - %s", arr[1])
	}

	return urls, nil
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
