package tunnel

import (
	"net"
	"strconv"
	"strings"

	"github.com/ilaziness/vela/internal/usererr"
)

// ValidateListenAddr checks a local listen address (IP:port). The host
// must be a literal IP so a typo cannot silently bind a name that
// resolves differently per platform.
func ValidateListenAddr(addr string) error {
	host, port, err := splitAddr(addr, "validation.addr_ip_port", "Format: IP:port")
	if err != nil {
		return err
	}
	if host == "" {
		return usererr.Coded("validation.addr_required", "Address is required")
	}
	if net.ParseIP(host) == nil {
		return usererr.Coded("validation.ip_invalid", "Invalid IP address")
	}
	return validatePortText(port)
}

// ValidateTargetAddr checks a forward target (host:port). The host may
// be an IP or a hostname; the port is required.
func ValidateTargetAddr(addr string) error {
	host, port, err := splitAddr(addr, "validation.addr_host_port", "Format: host:port")
	if err != nil {
		return err
	}
	if host == "" || strings.ContainsAny(host, " \t") {
		return usererr.Coded("validation.addr_host_port", "Format: host:port")
	}
	return validatePortText(port)
}

// ValidatePort checks a 1-65535 port number.
func ValidatePort(port int) error {
	if port == 0 {
		return usererr.Coded("validation.port_required", "Port is required")
	}
	if port < 1 || port > 65535 {
		return usererr.Coded("validation.port_range", "Port must be 1-65535")
	}
	return nil
}

func splitAddr(addr, code, fallback string) (host, port string, err error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", "", usererr.Coded("validation.addr_required", "Address is required")
	}
	host, port, err = net.SplitHostPort(addr)
	if err != nil {
		return "", "", usererr.Coded(code, fallback)
	}
	if strings.TrimSpace(port) == "" {
		return "", "", usererr.Coded("validation.port_required", "Port is required")
	}
	return host, port, nil
}

func validatePortText(port string) error {
	n, err := strconv.Atoi(strings.TrimSpace(port))
	if err != nil {
		return usererr.Coded("validation.port_range", "Port must be 1-65535")
	}
	return ValidatePort(n)
}
