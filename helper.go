package DomainSentinel

import (
	"net"
	"net/http"
	"strings"
)

func getClientIP(req *http.Request) string {
	ip, _, err := net.SplitHostPort(req.RemoteAddr)
	if err == nil && net.ParseIP(ip) != nil {
		return ip
	}
	return "" // unknown
}

func sanitizeHeader(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}
