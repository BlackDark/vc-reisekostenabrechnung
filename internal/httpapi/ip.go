package httpapi

import (
	"net"
	"net/http"
	"strings"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/config"
)

func peerIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return net.ParseIP(host)
}

// ClientIP returns the caller address. X-Forwarded-For is used only when the
// direct peer is in TRUSTED_PROXIES; the rightmost untrusted hop wins.
func ClientIP(r *http.Request, trusted []*net.IPNet) string {
	peer := peerIP(r.RemoteAddr)
	if peer == nil {
		return ""
	}
	if !config.IPTrusted(trusted, peer) {
		return peer.String()
	}
	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return peer.String()
	}
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip := net.ParseIP(strings.TrimSpace(parts[i]))
		if ip == nil {
			continue
		}
		if !config.IPTrusted(trusted, ip) {
			return ip.String()
		}
	}
	return peer.String()
}
