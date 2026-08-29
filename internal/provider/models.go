package provider

import (
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
