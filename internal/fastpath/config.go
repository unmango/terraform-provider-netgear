package fastpath

import (
	"slices"
	"strconv"
	"strings"
)

// RunningConfig is the parsed form of `show running-config`.
type RunningConfig struct {
	VLANs      map[int64]*VLAN
	Interfaces map[string]*Interface
	LAGs       map[int64]*LAG
}

// LAG is a link aggregation group, with the members gathered from the interface
// blocks whose ports were added to it.
type LAG struct {
	ID   int64
	Name string

	// InterfaceID is how the rest of the config refers to the group, such as `3/1`.
	InterfaceID string

	// Mode is `lacp` when the group turns off port-channel static, `static`
	// otherwise, which is the switch's own default.
	Mode     string
	HashMode int64
	Shutdown bool
	Members  []string
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

	// LAG is the interface id of the group this port was added to, empty when the
	// port stands alone.
	LAG string
}

// ParseRunningConfig reads the output of `show running-config`. Unrecognized
// lines are ignored: the config carries a large amount of state this provider
// does not manage.
func ParseRunningConfig(config string) *RunningConfig {
	parsed := &RunningConfig{
		VLANs:      map[int64]*VLAN{},
		Interfaces: map[string]*Interface{},
		LAGs:       map[int64]*LAG{},
	}

	var (
		inDatabase bool
		current    *Interface
		currentLAG *LAG
	)

	for raw := range strings.SplitSeq(config, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "!") {
			continue
		}

		if line == "exit" {
			inDatabase = false
			current = nil
			currentLAG = nil
			continue
		}

		if line == "vlan database" {
			inDatabase = true
			current = nil
			currentLAG = nil
			continue
		}

		if field, ok := strings.CutPrefix(line, "interface lag "); ok {
			inDatabase = false
			current = nil
			currentLAG = nil
			if id, ok := parseID(field); ok {
				currentLAG = parsed.lag(id)
			}
			continue
		}

		if port, ok := strings.CutPrefix(line, "interface "); ok {
			// The config prints the short spelling, `interface g7`.
			port = NormalizePort(port)
			inDatabase = false
			currentLAG = nil
			current = parsed.iface(port)
			continue
		}

		switch {
		case inDatabase:
			parsed.parseDatabaseLine(line)
		case currentLAG != nil:
			parseLAGLine(currentLAG, line)
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

func (c *RunningConfig) lag(id int64) *LAG {
	if existing, ok := c.LAGs[id]; ok {
		return existing
	}

	lag := &LAG{
		ID: id,
		// FASTPATH numbers LAG interfaces in unit 3 on this hardware. A member port
		// that names the group differently overrides this during resolution.
		InterfaceID: "3/" + strconv.FormatInt(id, 10),
		Mode:        "static",
	}
	c.LAGs[id] = lag

	return lag
}

// LAG returns the group with the given id, and whether the config defines it.
func (c *RunningConfig) LAG(id int64) (*LAG, bool) {
	lag, ok := c.LAGs[id]
	return lag, ok
}

func parseLAGLine(lag *LAG, line string) {
	fields := strings.Fields(line)

	switch {
	case fields[0] == "description":
		lag.Name = unquote(strings.Join(fields[1:], " "))
	case line == "no port-channel static":
		lag.Mode = "lacp"
	case line == "port-channel static":
		lag.Mode = "static"
	case line == "shutdown":
		lag.Shutdown = true
	case strings.HasPrefix(line, "port-channel load-balance ") && len(fields) > 2:
		if mode, ok := parseID(fields[2]); ok {
			lag.HashMode = mode
		}
	}
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
	case fields[0] == "addport" && len(fields) > 1:
		iface.LAG = NormalizePort(strings.Join(fields[1:], " "))
	}
}

// resolveMembership folds the per interface participation and addport lines back
// onto the VLANs and LAGs they reference, which is how the provider models
// membership. Ports are visited in a stable order so the resulting lists do not
// depend on map iteration.
func (c *RunningConfig) resolveMembership() {
	ports := make([]string, 0, len(c.Interfaces))
	for port := range c.Interfaces {
		ports = append(ports, port)
	}
	slices.Sort(ports)

	for _, port := range ports {
		iface := c.Interfaces[port]

		if iface.LAG != "" {
			if id, ok := lagIDFromInterface(iface.LAG); ok {
				lag := c.lag(id)
				lag.InterfaceID = iface.LAG
				lag.Members = append(lag.Members, iface.Port)
			}
		}

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

// lagIDFromInterface reads the group number off a LAG interface id such as `3/1`.
func lagIDFromInterface(interfaceID string) (int64, bool) {
	_, number, found := strings.Cut(interfaceID, "/")
	if !found {
		return 0, false
	}

	return parseID(number)
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
