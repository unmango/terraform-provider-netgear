package fastpath

import (
	"regexp"
	"strings"
)

// PortStatus is the operational state of a port, from `show interfaces status`.
type PortStatus struct {
	// AdminStatus is `enable` or `disable`, empty when the switch did not report it.
	AdminStatus string

	// LinkStatus is `up` or `down`, empty when the switch did not report it.
	LinkStatus string
}

// portIDRe matches the FASTPATH port ids that begin a status row, covering both
// the `0/1` and `1/0/1` notations.
var portIDRe = regexp.MustCompile(`^\d+/\d+(/\d+)?$`)

// ParseInterfaceStatus reads the output of `show interfaces status`. The layout is
// column formatted and varies by firmware, so rows are read by token: a row starts
// with a port id, the admin mode is whichever field reads Enable or Disable, and
// the link state is the field after it. Anything unrecognized yields an empty
// PortStatus rather than an error.
func ParseInterfaceStatus(output string) map[string]PortStatus {
	statuses := map[string]PortStatus{}

	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(strings.TrimSuffix(line, "\r"))
		if len(fields) == 0 || !portIDRe.MatchString(fields[0]) {
			continue
		}

		statuses[fields[0]] = parseStatusFields(fields[1:])
	}

	return statuses
}

// Known reports whether the switch said anything about the port at all.
func (s PortStatus) Known() bool {
	return s.AdminStatus != "" || s.LinkStatus != ""
}

func parseStatusFields(fields []string) PortStatus {
	var status PortStatus

	for i, field := range fields {
		switch strings.ToLower(field) {
		case "enable", "enabled":
			status.AdminStatus = "enable"
		case "disable", "disabled":
			status.AdminStatus = "disable"
		case "up", "down":
			// The link state follows the admin mode. Taking the first up or down
			// after it avoids the media and flow control columns further right.
			if status.AdminStatus != "" && status.LinkStatus == "" {
				status.LinkStatus = strings.ToLower(field)
			}
			continue
		default:
			continue
		}

		// A link state immediately after the admin mode is the common layout.
		if i+1 < len(fields) && status.LinkStatus == "" {
			switch strings.ToLower(fields[i+1]) {
			case "up", "down":
				status.LinkStatus = strings.ToLower(fields[i+1])
			}
		}
	}

	return status
}
