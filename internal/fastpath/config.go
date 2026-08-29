package fastpath

import (
	"strconv"
	"strings"
)

// RunningConfig is the parsed form of `show running-config`.
type RunningConfig struct {
	VLANs      map[int64]*VLAN
	Interfaces map[string]*Interface
}

// VLAN is an entry from the `vlan database` block, with the membership gathered
// from the interface blocks that reference it.
type VLAN struct {
	ID      int64
	Name    string
	Routing bool

	// Tagged and Untagged hold port ids in the order the config lists them.
	Tagged   []string
	Untagged []string
}

// Interface is the configuration of one port.
type Interface struct {
	Port        string
	Description string
	Shutdown    bool
	Speed       string
	PVID        int64
	MTU         int64

	// Participation and Tagging record the VLANs the port joins, and which of
	// those it carries tagged.
	Participation []int64
	Tagging       []int64
}

// ParseRunningConfig reads the output of `show running-config`. Unrecognized
// lines are ignored: the config carries a large amount of state this provider
// does not manage.
func ParseRunningConfig(config string) *RunningConfig {
	parsed := &RunningConfig{
		VLANs:      map[int64]*VLAN{},
		Interfaces: map[string]*Interface{},
	}

	var (
		inDatabase bool
		current    *Interface
	)

	for raw := range strings.SplitSeq(config, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "!") {
			continue
		}

		if line == "exit" {
			inDatabase = false
			current = nil
			continue
		}

		if line == "vlan database" {
			inDatabase = true
			current = nil
			continue
		}

		if port, ok := strings.CutPrefix(line, "interface "); ok {
			port = strings.TrimSpace(port)
			inDatabase = false
			current = parsed.iface(port)
			continue
		}

		switch {
		case inDatabase:
			parsed.parseDatabaseLine(line)
		case current != nil:
			parsed.parseInterfaceLine(current, line)
		}
	}

	parsed.resolveMembership()

	return parsed
}

// VLAN returns the VLAN with the given id, and whether the config defines it.
func (c *RunningConfig) VLAN(id int64) (*VLAN, bool) {
	vlan, ok := c.VLANs[id]
	return vlan, ok
}

// Interface returns the port with the given id, and whether the config defines it.
func (c *RunningConfig) Interface(port string) (*Interface, bool) {
	iface, ok := c.Interfaces[port]
	return iface, ok
}

func (c *RunningConfig) iface(port string) *Interface {
	if existing, ok := c.Interfaces[port]; ok {
		return existing
	}

	iface := &Interface{Port: port}
	c.Interfaces[port] = iface

	return iface
}

func (c *RunningConfig) vlan(id int64) *VLAN {
	if existing, ok := c.VLANs[id]; ok {
		return existing
	}

	vlan := &VLAN{ID: id}
	c.VLANs[id] = vlan

	return vlan
}

func (c *RunningConfig) parseDatabaseLine(line string) {
	fields := strings.Fields(line)
	if len(fields) < 2 || fields[0] != "vlan" {
		return
	}

	switch fields[1] {
	case "name":
		// vlan name 10 "mgmt"
		if len(fields) < 4 {
			return
		}
		if id, ok := parseID(fields[2]); ok {
			c.vlan(id).Name = unquote(strings.Join(fields[3:], " "))
		}
	case "routing":
		// vlan routing 10
		if id, ok := parseID(fields[2]); ok {
			c.vlan(id).Routing = true
		}
	default:
		// vlan 10 or vlan 10,20,30
		for _, id := range parseIDList(fields[1]) {
			c.vlan(id)
		}
	}
}

func (c *RunningConfig) parseInterfaceLine(iface *Interface, line string) {
	fields := strings.Fields(line)

	switch {
	case fields[0] == "description":
		iface.Description = unquote(strings.Join(fields[1:], " "))
	case line == "shutdown":
		iface.Shutdown = true
	case fields[0] == "mtu" && len(fields) > 1:
		if mtu, ok := parseID(fields[1]); ok {
			iface.MTU = mtu
		}
	case fields[0] == "speed" && len(fields) > 1:
		iface.Speed = strings.Join(fields[1:], "-")
	case strings.HasPrefix(line, "vlan pvid ") && len(fields) > 2:
		if pvid, ok := parseID(fields[2]); ok {
			iface.PVID = pvid
		}
	case strings.HasPrefix(line, "vlan participation include ") && len(fields) > 3:
		iface.Participation = append(iface.Participation, parseIDList(fields[3])...)
	case strings.HasPrefix(line, "vlan tagging ") && len(fields) > 2:
		iface.Tagging = append(iface.Tagging, parseIDList(fields[2])...)
	}
}

// resolveMembership folds the per interface participation lines back onto the
// VLANs they reference, which is how the provider models membership.
func (c *RunningConfig) resolveMembership() {
	for _, iface := range c.Interfaces {
		tagged := map[int64]bool{}
		for _, id := range iface.Tagging {
			tagged[id] = true
		}

		for _, id := range iface.Participation {
			vlan := c.vlan(id)
			if tagged[id] {
				vlan.Tagged = append(vlan.Tagged, iface.Port)
			} else {
				vlan.Untagged = append(vlan.Untagged, iface.Port)
			}
		}
	}
}

func parseID(field string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(field), 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

// parseIDList reads the comma separated and hyphenated id lists FASTPATH accepts,
// such as "10", "10,20" or "10-12".
func parseIDList(field string) []int64 {
	var ids []int64

	for part := range strings.SplitSeq(field, ",") {
		lo, hi, isRange := strings.Cut(part, "-")
		start, ok := parseID(lo)
		if !ok {
			continue
		}

		if !isRange {
			ids = append(ids, start)
			continue
		}

		end, ok := parseID(hi)
		if !ok {
			continue
		}
		for id := start; id <= end; id++ {
			ids = append(ids, id)
		}
	}

	return ids
}

func unquote(value string) string {
	value = strings.TrimSpace(value)
	for _, quote := range []string{`"`, `'`} {
		if len(value) >= 2 && strings.HasPrefix(value, quote) && strings.HasSuffix(value, quote) {
			return value[1 : len(value)-1]
		}
	}
	return value
}
