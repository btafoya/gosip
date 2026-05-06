package sip

import "strings"

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
