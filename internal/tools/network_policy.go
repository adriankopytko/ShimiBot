package tools

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

const allowPrivateEgressEnv = "SHIMIBOT_ALLOW_PRIVATE_EGRESS"

var resolveIPAddrs = net.DefaultResolver.LookupIPAddr

func EnsureOutboundURLAllowed(ctx context.Context, rawURL string) error {
	if strings.EqualFold(strings.TrimSpace(os.Getenv(allowPrivateEgressEnv)), "true") {
		return nil
	}

	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	scheme := strings.ToLower(strings.TrimSpace(parsedURL.Scheme))
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported URL scheme %q", parsedURL.Scheme)
	}

	hostname := strings.TrimSpace(parsedURL.Hostname())
	if hostname == "" {
		return fmt.Errorf("invalid URL host")
	}
	if isLocalHostname(hostname) {
		return fmt.Errorf("blocked outbound host")
	}

	if ip := net.ParseIP(hostname); ip != nil {
		if isPrivateOrLocalIP(ip) {
			return fmt.Errorf("blocked outbound IP")
		}
		return nil
	}

	ipAddrs, err := resolveIPAddrs(ctx, hostname)
	if err != nil {
		return fmt.Errorf("failed resolving host: %w", err)
	}
	if len(ipAddrs) == 0 {
		return fmt.Errorf("failed resolving host: no addresses")
	}

	for _, ipAddr := range ipAddrs {
		if isPrivateOrLocalIP(ipAddr.IP) {
			return fmt.Errorf("blocked outbound host")
		}
	}

	return nil
}

func isLocalHostname(hostname string) bool {
	normalized := strings.ToLower(strings.TrimSpace(hostname))
	return normalized == "localhost" || strings.HasSuffix(normalized, ".localhost")
}

func isPrivateOrLocalIP(ip net.IP) bool {
	if ip == nil {
		return true
	}

	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}

	if ipv4 := ip.To4(); ipv4 != nil {
		if ipv4[0] == 169 && ipv4[1] == 254 {
			return true
		}
		if ipv4[0] == 100 && ipv4[1] >= 64 && ipv4[1] <= 127 {
			return true
		}
	}

	return false
}
