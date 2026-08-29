package fastpath

import (
	"strings"
)

// PortStatus is the operational state of a port.
type PortStatus struct {
	// AdminStatus is `enable` or `disable`, empty when the output has no admin
	// column. `show port` reports it, `show interfaces status` does not.
	AdminStatus string

	// LinkStatus is `up` or `down`, empty when the switch did not report it.
	LinkStatus string
}

// Known reports whether the switch said anything about the port at all.
func (s PortStatus) Known() bool {
	return s.AdminStatus != "" || s.LinkStatus != ""
}

// ParsePortStatus reads a status table, keyed by port id in the slot form. It
// handles both `show port`, which carries an admin mode column, and
// `show interfaces status`, which does not.
//
// The layout is column formatted and varies by firmware, so rows are read by
// token: a row starts with a port id, the admin mode is the first field reading
// Enable or Disable, and the link state is the first Up or Down after it.
// Anything unrecognized yields an empty PortStatus rather than an error.
func ParsePortStatus(output string) map[string]PortStatus {
	statuses := map[string]PortStatus{}

	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(strings.TrimSuffix(line, "\r"))

		port, rest, ok := cutPortID(fields)
		if !ok {
			continue
		}

		statuses[port] = parseStatusFields(rest)
	}

	return statuses
}

// cutPortID takes the port id off the front of a row. The short LAG spelling is
// two fields, `lag 1`, so it cannot be read off the first field alone.
func cutPortID(fields []string) (string, []string, bool) {
	if len(fields) == 0 {
		return "", nil, false
	}

	if strings.EqualFold(fields[0], "lag") && len(fields) > 1 {
		id := fields[0] + " " + fields[1]
		if IsPortID(id) {
			return NormalizePort(id), fields[2:], true
		}
		return "", nil, false
	}

	if !IsPortID(fields[0]) {
		return "", nil, false
	}

	return NormalizePort(fields[0]), fields[1:], true
}

func parseStatusFields(fields []string) PortStatus {
	var status PortStatus

	for _, field := range fields {
		switch strings.ToLower(field) {
		case "enable", "enabled":
			if status.AdminStatus == "" {
				status.AdminStatus = "enable"
			}
		case "disable", "disabled":
			if status.AdminStatus == "" {
				status.AdminStatus = "disable"
			}
		case "up", "down":
			if status.LinkStatus == "" {
				status.LinkStatus = strings.ToLower(field)
			}
		}
	}

	return status
}
