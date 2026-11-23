package handler

import (
	"net/url"
	"strings"
)

// webLink accepts an empty link or a plain http(s) URL with a host. Anything else, like
// javascript: or data: URLs, is refused, because these links are rendered as <a href>.
func webLink(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", true
	}
	if len(s) > 300 {
		return "", false
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return "", false
	}
	return s, true
}
