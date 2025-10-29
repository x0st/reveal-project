package core

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
)

func IPParseRanges(rangesStr string) ([]string, error) {
	ranges := strings.Split(rangesStr, ",")
	var allIPs []string

	for _, r := range ranges {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}

		var ips []string
		var err error

		if strings.Contains(r, "/") {
			// CIDR notation
			ips, err = ipParseCIDR(r)
		} else if strings.Contains(r, "-") {
			// Hyphen range
			ips, err = ipParseHyphenRange(r)
		} else {
			// Single IP
			if net.ParseIP(r) == nil {
				return nil, fmt.Errorf("invalid IP: %s", r)
			}
			ips = []string{r}
		}

		if err != nil {
			return nil, err
		}
		allIPs = append(allIPs, ips...)
	}

	return allIPs, nil
}

func ipParseCIDR(cidr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR %s: %v", cidr, err)
	}

	incrementIp := func(ip net.IP) {
		for j := len(ip) - 1; j >= 0; j-- {
			ip[j]++
			if ip[j] > 0 {
				break
			}
		}
	}

	var ips []string
	for ithIP := ip.Mask(ipnet.Mask); ipnet.Contains(ithIP); incrementIp(ithIP) {
		ips = append(ips, ithIP.String())
	}

	// Remove network and broadcast addresses for subnets (unless it's a /32 or /31)
	if len(ips) > 2 {
		return ips[1 : len(ips)-1], nil
	}

	return ips, nil
}

func ipParseHyphenRange(rangeStr string) ([]string, error) {
	parts := strings.Split(rangeStr, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid hyphen range format: %s", rangeStr)
	}

	startIP := net.ParseIP(strings.TrimSpace(parts[0]))
	endIP := net.ParseIP(strings.TrimSpace(parts[1]))

	if startIP == nil || endIP == nil {
		return nil, fmt.Errorf("invalid IP in range: %s", rangeStr)
	}

	// Convert to IPv4
	startIP = startIP.To4()
	endIP = endIP.To4()

	if startIP == nil || endIP == nil {
		return nil, fmt.Errorf("only IPv4 ranges supported: %s", rangeStr)
	}

	// Convert IPs to uint32 for easier iteration
	start := binary.BigEndian.Uint32(startIP)
	end := binary.BigEndian.Uint32(endIP)

	if start > end {
		return nil, fmt.Errorf("start IP must be <= end IP: %s", rangeStr)
	}

	var ips []string
	for i := start; i <= end; i++ {
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, i)
		ips = append(ips, ip.String())
	}

	return ips, nil
}
