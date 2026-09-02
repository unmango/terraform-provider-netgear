package provider

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// stringSet builds a set attribute value from port ids, in a stable order.
func stringSet(values []string) types.Set {
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	sorted = slices.Compact(sorted)

	elements := make([]attr.Value, 0, len(sorted))
	for _, value := range sorted {
		elements = append(elements, types.StringValue(value))
	}

	return types.SetValueMust(types.StringType, elements)
}

// setOrNull keeps an attribute null when the switch reports nothing for it and
// the configuration never set it, so reading does not invent an empty set.
func setOrNull(prior types.Set, values []string) types.Set {
	if len(values) == 0 && prior.IsNull() {
		return prior
	}

	return stringSet(values)
}

// stringOrNull keeps an attribute null when the switch reports an empty value and
// the configuration never set it.
func stringOrNull(prior types.String, value string) types.String {
	if value == "" && prior.IsNull() {
		return prior
	}

	return types.StringValue(value)
}

// int64OrNull keeps an attribute null when the switch reports zero for it and the
// configuration never set it.
func int64OrNull(prior types.Int64, value int64) types.Int64 {
	if value == 0 && prior.IsNull() {
		return prior
	}

	return types.Int64Value(value)
}

// quote wraps a value the way the FASTPATH CLI expects free text arguments.
func quote(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, "") + `"`
}

// itoa renders an id for use in a command.
func itoa(value int64) string {
	return strconv.FormatInt(value, 10)
}

// sortedKeys returns the members of a port lookup in a stable order, so the
// commands a resource sends do not depend on map iteration.
func sortedKeys(ports map[string]struct{}) []string {
	keys := make([]string, 0, len(ports))
	for port := range ports {
		keys = append(keys, port)
	}
	slices.Sort(keys)

	return keys
}

// int64Set builds a set attribute value from vlan ids, in a stable order.
func int64Set(values []int64) types.Set {
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	sorted = slices.Compact(sorted)

	elements := make([]attr.Value, 0, len(sorted))
	for _, value := range sorted {
		elements = append(elements, types.Int64Value(value))
	}

	return types.SetValueMust(types.Int64Type, elements)
}

// sortedPorts orders port ids the way the switch numbers them, so `0/10` follows
// `0/9` rather than `0/1`. Ids that are not in the slot form sort last, by name.
func sortedPorts(ports []string) []string {
	sorted := slices.Clone(ports)
	slices.SortFunc(sorted, func(a, b string) int {
		unitA, portA, okA := splitPort(a)
		unitB, portB, okB := splitPort(b)

		switch {
		case okA && okB:
			return cmp.Or(cmp.Compare(unitA, unitB), cmp.Compare(portA, portB))
		case okA:
			return -1
		case okB:
			return 1
		default:
			return strings.Compare(a, b)
		}
	})

	return sorted
}

// splitPort reads the unit and port halves of an id in the slot form.
func splitPort(id string) (int64, int64, bool) {
	unit, port, found := strings.Cut(id, "/")
	if !found {
		return 0, 0, false
	}

	u, err := strconv.ParseInt(unit, 10, 64)
	if err != nil {
		return 0, 0, false
	}

	p, err := strconv.ParseInt(port, 10, 64)
	if err != nil {
		return 0, 0, false
	}

	return u, p, true
}
