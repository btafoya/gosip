package sip

import (
	"net"
	"strings"
)

// sanitizeSIPHeaderValue sanitizes a string for use in SIP header values.
// It strips newlines, angle brackets, and control characters to prevent
// header injection attacks.
func sanitizeSIPHeaderValue(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\r' || r == '\n':
			// Strip newlines
			continue
		case r == '<' || r == '>':
			// Strip angle brackets
			continue
		case r < 0x20 || r == 0x7f:
			// Strip control characters and DEL
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// twilioSignalingCIDRs lists Twilio's public Elastic SIP Trunking signaling IP
// ranges per https://www.twilio.com/docs/sip-trunking/ip-addresses.
// Use this for forensic logging, NOT auth — Twilio publishes these and bad actors
// can route through AWS too. Real authentication uses TLS cert pinning,
// per-trunk min_attestation, or Twilio webhook signature validation.
var twilioSignalingCIDRs = func() []*net.IPNet {
	cidrs := []string{
		"54.172.60.0/30",
		"54.244.51.0/30",
		"54.171.127.192/30",
		"35.156.191.128/30",
		"54.252.254.64/30",
		"177.71.206.192/30",
		"54.65.63.192/30",
		"54.169.127.128/30",
		"54.252.254.64/30",
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		if _, n, err := net.ParseCIDR(c); err == nil {
			nets = append(nets, n)
		}
	}
	return nets
}()

// isTwilioSignalingIP returns true when ip falls within Twilio's published
// SIP signaling CIDR ranges. Forensic hint only — do not gate auth on this.
func isTwilioSignalingIP(ip string) bool {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return false
	}
	for _, n := range twilioSignalingCIDRs {
		if n.Contains(parsed) {
			return true
		}
	}
	return false
}
