package fast

import "testing"

func TestIsAllowedHost(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://fast.com/app-abc123.js", true},
		{"https://api.fast.com/netflix/speedtest", true},
		{"https://ipv4-c001.1.oca.nflxvideo.net/speedtest", true},
		{"http://fast.com", true},
		{"https://evil.com/app-abc123.js", false},
		{"https://fast.com.evil.com/", false},
		{"https://169.254.169.254/latest/meta-data/", false},
		{"not a url", false},
	}

	for _, c := range cases {
		if got := isAllowedHost(c.url); got != c.want {
			t.Errorf("isAllowedHost(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}
