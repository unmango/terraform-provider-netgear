package provider

import (
	"context"
	"maps"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/unmango/terraform-provider-netgear/internal/fastpath"
)

var (
	_ datasource.DataSource              = &interfacesDataSource{}
	_ datasource.DataSourceWithConfigure = &interfacesDataSource{}
)

type interfacesDataSource struct {
	data *switchData
}

type interfacesDataSourceModel struct {
	ID         types.String          `tfsdk:"id"`
	Interfaces []interfaceEntryModel `tfsdk:"interfaces"`
}

func NewInterfacesDataSource() datasource.DataSource {
	return &interfacesDataSource{}
}

func (d *interfacesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_interfaces"
}

func (d *interfacesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Every physical port on the switch, its configuration from " +
			"`show running-config` and its live state from `show port`.\n\n" +
			"The port list comes from `show port`, not the running config, so ports left entirely " +
			"at their defaults are included. LAG interfaces are reported by `netgear_lags` instead.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Constant `interfaces`. The framework requires an id.",
				Computed:            true,
			},
			"interfaces": schema.ListNestedAttribute{
				MarkdownDescription: "The ports, in ascending port order.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: interfaceEntryAttributes(),
				},
			},
		},
	}
}

func (d *interfacesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.data = switchFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *interfacesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.data == nil {
		resp.Diagnostics.Append(errNotConfigured("netgear_interfaces")...)
		return
	}

	running, err := d.data.runningConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read the Switch Configuration", err.Error())
		return
	}

	status, err := d.data.readAllStatus(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read the Port Status", err.Error())
		return
	}

	state := interfacesDataSourceModel{
		ID:         types.StringValue("interfaces"),
		Interfaces: []interfaceEntryModel{},
	}
	for _, port := range sortedPorts(slices.Collect(maps.Keys(status))) {
		if fastpath.IsLAG(port) {
			continue
		}

		iface, configured := running.Interface(port)
		if !configured {
			iface = &fastpath.Interface{Port: port}
		}

		state.Interfaces = append(state.Interfaces, interfaceEntry(iface, status[port]))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
