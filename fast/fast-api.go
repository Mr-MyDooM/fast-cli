package fast

import "fmt"
import "bytes"
import "net/http"
import "io"
import "regexp"
import "github.com/gesquive/cli"

// UseHTTPS sets if HTTPS is used
var UseHTTPS = true

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
	re := regexp.MustCompile("app-.*\\.js")
	scriptNames := re.FindAllString(fastBody, 1)
	if len(scriptNames) == 0 {
		return "", fmt.Errorf("could not find fast.com app script, page layout may have changed")
	}

	scriptURL := fmt.Sprintf("%s/%s", baseURL, scriptNames[0])
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

func getPage(url string) (contents string, err error) {
	// Create the string buffer
	buffer := bytes.NewBuffer(nil)

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return contents, err
	}
	defer resp.Body.Close()

	// Writer the body to file
	_, err = io.Copy(buffer, resp.Body)
	if err != nil {
		return contents, err
	}
	contents = buffer.String()

	return
}
