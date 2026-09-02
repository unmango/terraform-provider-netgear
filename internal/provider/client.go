package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/UnstoppableMango/terraform-provider-netgear/internal/fastpath"
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

// switchData is what the provider hands to each resource: the client plus the
// settings a resource needs to honour.
type switchData struct {
	client     Client
	saveConfig bool
}

// apply runs commands and then persists them to NVRAM when the provider is
// configured to. FASTPATH applies changes live, so a change that is not saved is
// lost at the next reboot.
func (d *switchData) apply(ctx context.Context, cmds ...string) (string, error) {
	out, err := d.client.Run(ctx, cmds...)
	if err != nil {
		return out, err
	}

	if !d.saveConfig {
		return out, nil
	}

	return out, d.client.Save(ctx)
}

// runningConfig reads and parses `show running-config`.
func (d *switchData) runningConfig(ctx context.Context) (*fastpath.RunningConfig, error) {
	out, err := d.client.RunningConfig(ctx)
	if err != nil {
		return nil, err
	}

	return fastpath.ParseRunningConfig(out), nil
}

// readStatus reports the operational state of one port. `show port` is the source
// rather than `show interfaces status`, because only it carries the admin mode.
func (d *switchData) readStatus(ctx context.Context, port string) (fastpath.PortStatus, error) {
	out, err := d.client.Run(ctx, "show port "+port)
	if err != nil {
		return fastpath.PortStatus{}, err
	}

	return fastpath.ParsePortStatus(out)[fastpath.NormalizePort(port)], nil
}

// readAllStatus reports the operational state of every port the switch knows,
// keyed by port id in the slot form. LAG interfaces appear alongside physical
// ports; `fastpath.IsLAG` tells them apart.
func (d *switchData) readAllStatus(ctx context.Context) (map[string]fastpath.PortStatus, error) {
	out, err := d.client.Run(ctx, "show port all")
	if err != nil {
		return nil, err
	}

	return fastpath.ParsePortStatus(out), nil
}

// switchFromProviderData extracts the configured switch from resource or data
// source provider data. It returns nil when the framework has not configured the
// provider yet, which is expected during validation and planning.
func switchFromProviderData(providerData any, diags *diag.Diagnostics) *switchData {
	if providerData == nil {
		return nil
	}

	data, ok := providerData.(*switchData)
	if !ok {
		diags.AddError(
			"Unexpected Provider Data Type",
			fmt.Sprintf("Expected *provider.switchData, got %T. Please report this to the provider developers.", providerData),
		)
		return nil
	}

	return data
}

// errNotConfigured is the diagnostic a resource reports when it has no client,
// which means provider configuration did not complete.
func errNotConfigured(resourceType string) diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddError(
		"Provider Not Configured",
		fmt.Sprintf("%s has no switch connection. Check the provider block for errors reported above.", resourceType),
	)
	return diags
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
