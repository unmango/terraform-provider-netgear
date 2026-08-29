package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Client is the FASTPATH CLI surface the resources depend on.
// It is defined here rather than in the client package so tests can supply a fake.
type Client interface {
	// Run executes commands in sequence and returns the combined session output.
	Run(ctx context.Context, cmds ...string) (string, error)

	// RunningConfig returns the output of `show running-config`.
	RunningConfig(ctx context.Context) (string, error)

	// Save persists the running configuration to NVRAM.
	Save(ctx context.Context) error
}

// prober is the optional half of Client that checks the switch is reachable and
// the credentials work, used once during provider configuration.
type prober interface {
	Probe(ctx context.Context) error
}

// clientFromProviderData extracts the configured Client from resource or data source
// provider data. It returns nil when the framework has not configured the provider yet,
// which is expected during validation and planning.
func clientFromProviderData(providerData any, diags *diag.Diagnostics) Client {
	if providerData == nil {
		return nil
	}

	client, ok := providerData.(Client)
	if !ok {
		diags.AddError(
			"Unexpected Provider Data Type",
			fmt.Sprintf("Expected provider.Client, got %T. Please report this to the provider developers.", providerData),
		)
		return nil
	}

	return client
}

// portSet converts a set of FASTPATH port ids into a lookup keyed by port. A null
// or unknown set yields an empty lookup.
func portSet(ctx context.Context, set types.Set, diags *diag.Diagnostics) (map[string]struct{}, bool) {
	ports := map[string]struct{}{}
	if set.IsNull() || set.IsUnknown() {
		return ports, true
	}

	var elements []string
	diags.Append(set.ElementsAs(ctx, &elements, false)...)
	if diags.HasError() {
		return nil, false
	}

	for _, port := range elements {
		ports[port] = struct{}{}
	}

	return ports, true
}

// notImplemented reports that a CRUD method has a schema but no FASTPATH client
// behind it yet.
func notImplemented(resourceType, method string) diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddError(
		"Not Implemented",
		fmt.Sprintf("%s %s is not implemented: the FASTPATH CLI client is still to be written.", resourceType, method),
	)
	return diags
}
