package provider

import (
	"context"
	"maps"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = &vlansDataSource{}
	_ datasource.DataSourceWithConfigure = &vlansDataSource{}
)

type vlansDataSource struct {
	data *switchData
}

type vlansDataSourceModel struct {
	ID    types.String     `tfsdk:"id"`
	Vlans []vlanEntryModel `tfsdk:"vlans"`
}

func NewVlansDataSource() datasource.DataSource {
	return &vlansDataSource{}
}

func (d *vlansDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vlans"
}

func (d *vlansDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Every VLAN the switch has configured, read from the `vlan database` block of " +
			"`show running-config`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Constant `vlans`. The framework requires an id.",
				Computed:            true,
			},
			"vlans": schema.ListNestedAttribute{
				MarkdownDescription: "The VLANs, in ascending id order.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: vlanEntryAttributes(),
				},
			},
		},
	}
}

func (d *vlansDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.data = switchFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *vlansDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.data == nil {
		resp.Diagnostics.Append(errNotConfigured("netgear_vlans")...)
		return
	}

	running, err := d.data.runningConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read the Switch Configuration", err.Error())
		return
	}

	state := vlansDataSourceModel{
		ID:    types.StringValue("vlans"),
		Vlans: []vlanEntryModel{},
	}
	for _, id := range slices.Sorted(maps.Keys(running.VLANs)) {
		state.Vlans = append(state.Vlans, vlanEntry(running.VLANs[id]))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
