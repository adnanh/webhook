package main

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// parsedTrustedProxies holds the parsed CIDR networks from the --trusted-proxies flag.
var parsedTrustedProxies []*net.IPNet
var parsedAccessWhitelist []*net.IPNet
var parsedAccessBlacklist []*net.IPNet

// initTrustedProxies parses the --trusted-proxies flag value into CIDR networks.
// Must be called after flag.Parse().
func parseIPNetworks(value, name string) ([]*net.IPNet, error) {
	var networks []*net.IPNet
	for _, entry := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' }) {
		if !strings.Contains(entry, "/") {
			ip := net.ParseIP(entry)
			if ip == nil {
				return nil, fmt.Errorf("invalid %s entry %q", name, entry)
			}
			if ip.To4() != nil {
				entry += "/32"
			} else {
				entry += "/128"
			}
		}
		_, cidr, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, fmt.Errorf("invalid %s entry %q: %w", name, entry, err)
		}
		networks = append(networks, cidr)
	}
	return networks, nil
}

func initTrustedProxies() error {
	var err error
	parsedTrustedProxies, err = parseIPNetworks(*trustedProxies, "trusted-proxies")
	return err
}

func initAccessControl() error {
	if strings.TrimSpace(*accessWhitelist) != "" && strings.TrimSpace(*accessBlacklist) != "" {
		return fmt.Errorf("access-whitelist and access-blacklist cannot be used together")
	}
	var err error
	parsedAccessWhitelist, err = parseIPNetworks(*accessWhitelist, "access-whitelist")
	if err != nil {
		return err
	}
	parsedAccessBlacklist, err = parseIPNetworks(*accessBlacklist, "access-blacklist")
	return err
}

func accessControlMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(resolveRealIP(r))
		allowed := true
		if len(parsedAccessWhitelist) > 0 {
			allowed = ip != nil && containsIP(parsedAccessWhitelist, ip)
		} else if len(parsedAccessBlacklist) > 0 {
			allowed = ip != nil && !containsIP(parsedAccessBlacklist, ip)
		}
		if !allowed {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("Access denied."))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func containsIP(networks []*net.IPNet, ip net.IP) bool {
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// isTrustedProxy checks whether the given remote address (IP:port) belongs to
// a trusted proxy network.
func isTrustedProxy(remoteAddr string) bool {
	if len(parsedTrustedProxies) == 0 {
		return false
	}

	ip := extractIP(remoteAddr)
	if ip == nil {
		return false
	}

	for _, cidr := range parsedTrustedProxies {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// resolveRealIP determines the real client IP address. If the request comes
// from a trusted proxy and --real-ip-header is configured, the IP is read from
// that header. Otherwise the TCP remote address is used.
func resolveRealIP(r *http.Request) string {
	if *realIPHeader != "" && isTrustedProxy(r.RemoteAddr) {
		headerVal := strings.TrimSpace(r.Header.Get(*realIPHeader))
		if headerVal != "" {
			// X-Forwarded-For may contain multiple IPs; take the first one.
			if idx := strings.IndexByte(headerVal, ','); idx != -1 {
				headerVal = strings.TrimSpace(headerVal[:idx])
			}
			return headerVal
		}
	}
	return r.RemoteAddr
}

// extractIP parses an IP from an address string that may include a port.
func extractIP(addr string) net.IP {
	addr = strings.Trim(addr, " []")

	// Try host:port split first.
	host, _, err := net.SplitHostPort(addr)
	if err == nil {
		return net.ParseIP(host)
	}

	return net.ParseIP(addr)
}
