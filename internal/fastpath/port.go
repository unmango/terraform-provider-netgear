package fastpath

import (
	"regexp"
	"strings"
)

// The CLI names the same port two ways. `show running-config` and `show port`
// print the short form, `g7` and `lag 1`, while `show interfaces status` prints
// the slot form, `0/7` and `3/1`. Commands accept either. The provider works in
// the slot form throughout, so everything read off the switch is normalized to it.
var (
	shortPortRe = regexp.MustCompile(`^g(\d+)$`)
	shortLagRe  = regexp.MustCompile(`^lag\s*(\d+)$`)
	slotPortRe  = regexp.MustCompile(`^\d+/\d+$`)
)

// lagUnit is the slot LAG interfaces live in on this hardware.
const lagUnit = "3/"

// NormalizePort converts a port id the switch printed into the slot form the
// provider uses. An id it does not recognize is returned unchanged, so an
// unfamiliar firmware degrades to passing the switch's own spelling through.
func NormalizePort(id string) string {
	id = strings.TrimSpace(id)

	if match := shortPortRe.FindStringSubmatch(id); match != nil {
		return "0/" + match[1]
	}

	if match := shortLagRe.FindStringSubmatch(strings.ToLower(id)); match != nil {
		return lagUnit + match[1]
	}

	return id
}

// IsPortID reports whether a field is a port id in either spelling, which is how
// a status row is told apart from a header or a rule.
func IsPortID(id string) bool {
	normalized := NormalizePort(id)

	return slotPortRe.MatchString(normalized)
}
