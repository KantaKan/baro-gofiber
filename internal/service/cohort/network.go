package cohort

import (
	"net"
	"strings"
)

// IPAllowed reports whether ip is inside any allowed CIDR or equal to any allowed IP.
// An empty list allows every network. Unparsable entries are ignored.
func IPAllowed(allowed []string, ip string) bool {
	if len(allowed) == 0 {
		return true
	}
	addr := net.ParseIP(strings.TrimSpace(ip))
	if addr == nil {
		return false
	}
	for _, entry := range allowed {
		entry = strings.TrimSpace(entry)
		if _, n, err := net.ParseCIDR(entry); err == nil {
			if n.Contains(addr) {
				return true
			}
			continue
		}
		if one := net.ParseIP(entry); one != nil && one.Equal(addr) {
			return true
		}
	}
	return false
}

func validNetworkEntry(entry string) bool {
	entry = strings.TrimSpace(entry)
	if _, _, err := net.ParseCIDR(entry); err == nil {
		return true
	}
	return net.ParseIP(entry) != nil
}
