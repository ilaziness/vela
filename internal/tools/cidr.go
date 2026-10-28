package tools

import (
	"fmt"
	"math/big"
	"net"
	"strings"

	"github.com/ilaziness/vela/internal/i18n"
)

// CIDRResult 子网计算结果
type CIDRResult struct {
	Success    bool   `json:"success"`
	Network    string `json:"network,omitempty"`
	Broadcast  string `json:"broadcast,omitempty"`
	Netmask    string `json:"netmask,omitempty"`
	PrefixLen  int    `json:"prefixLen,omitempty"`
	FirstHost  string `json:"firstHost,omitempty"`
	LastHost   string `json:"lastHost,omitempty"`
	HostCount  string `json:"hostCount,omitempty"`
	TotalAddrs string `json:"totalAddrs,omitempty"`
	Contains   *bool  `json:"contains,omitempty"`
	ContainsIP string `json:"containsIP,omitempty"`
	Error      string `json:"error,omitempty"`
}

// CalculateCIDR 解析 CIDR，可选检查 ip 是否在网段内
func (ts *Service) CalculateCIDR(cidr string, checkIP string) CIDRResult {
	cidr = strings.TrimSpace(cidr)
	if cidr == "" {
		return CIDRResult{Success: false, Error: i18n.T("tool.err.cidr_empty")}
	}

	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return CIDRResult{Success: false, Error: i18n.Tf("tool.err.invalid_cidr", err.Error())}
	}

	ones, bits := ipNet.Mask.Size()
	network := ipNet.IP.Mask(ipNet.Mask)

	result := CIDRResult{
		Success:   true,
		Network:   network.String() + "/" + fmt.Sprintf("%d", ones),
		Netmask:   net.IP(ipNet.Mask).String(),
		PrefixLen: ones,
	}

	total := new(big.Int).Lsh(big.NewInt(1), uint(bits-ones))
	result.TotalAddrs = total.String()

	switch {
	case ones == bits:
		// /32 或 /128：单主机
		result.Broadcast = network.String()
		result.FirstHost = network.String()
		result.LastHost = network.String()
		result.HostCount = "1"
	case bits == 32 && ones == 31:
		// /31 点对点
		first := network.String()
		last := lastIP(network, ipNet.Mask).String()
		result.Broadcast = last
		result.FirstHost = first
		result.LastHost = last
		result.HostCount = "2"
	case bits == 32:
		first := nextIP(network)
		bcast := lastIP(network, ipNet.Mask)
		last := prevIP(bcast)
		result.Broadcast = bcast.String()
		result.FirstHost = first.String()
		result.LastHost = last.String()
		hostCount := new(big.Int).Sub(total, big.NewInt(2))
		result.HostCount = hostCount.String()
	default:
		// IPv6：不单独算广播，可用范围取网络到末地址
		first := network
		last := lastIP(network, ipNet.Mask)
		result.Broadcast = last.String()
		result.FirstHost = first.String()
		result.LastHost = last.String()
		result.HostCount = total.String()
	}

	checkIP = strings.TrimSpace(checkIP)
	if checkIP != "" {
		parsed := net.ParseIP(checkIP)
		if parsed == nil {
			// 网段结果仍返回，错误说明检查 IP 无效
			result.Error = "Invalid check IP: " + checkIP
			return result
		}
		cidrIsV4 := network.To4() != nil
		checkIsV4 := parsed.To4() != nil
		if cidrIsV4 != checkIsV4 {
			result.Error = "Check IP address family does not match CIDR"
			return result
		}
		contains := ipNet.Contains(parsed)
		result.Contains = &contains
		result.ContainsIP = checkIP
	}

	return result
}

func nextIP(ip net.IP) net.IP {
	if v4 := ip.To4(); v4 != nil {
		n := new(big.Int).SetBytes(v4)
		n.Add(n, big.NewInt(1))
		return bigToIP(n, net.IPv4len)
	}
	n := new(big.Int).SetBytes(ip.To16())
	n.Add(n, big.NewInt(1))
	return bigToIP(n, net.IPv6len)
}

func prevIP(ip net.IP) net.IP {
	if v4 := ip.To4(); v4 != nil {
		n := new(big.Int).SetBytes(v4)
		n.Sub(n, big.NewInt(1))
		return bigToIP(n, net.IPv4len)
	}
	n := new(big.Int).SetBytes(ip.To16())
	n.Sub(n, big.NewInt(1))
	return bigToIP(n, net.IPv6len)
}

func lastIP(network net.IP, mask net.IPMask) net.IP {
	if v4 := network.To4(); v4 != nil && len(mask) == net.IPv4len {
		network = v4
	}
	if len(network) != len(mask) {
		out := make(net.IP, len(network))
		copy(out, network)
		return out
	}
	ip := make(net.IP, len(network))
	copy(ip, network)
	for i := range ip {
		ip[i] |= ^mask[i]
	}
	return ip
}

func bigToIP(n *big.Int, size int) net.IP {
	b := n.Bytes()
	if len(b) > size {
		b = b[len(b)-size:]
	}
	ip := make(net.IP, size)
	copy(ip[size-len(b):], b)
	return ip
}
