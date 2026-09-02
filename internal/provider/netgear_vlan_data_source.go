package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/UnstoppableMango/terraform-provider-netgear/internal/fastpath"
)

var (
	_ datasource.DataSource              = &vlanDataSource{}
	_ datasource.DataSourceWithConfigure = &vlanDataSource{}
)

type vlanDataSource struct {
	data *switchData
}

// vlanEntryModel is one VLAN as the switch reports it, shared by the singular and
// the plural data source.
type vlanEntryModel struct {
	VlanID        types.Int64  `tfsdk:"vlan_id"`
	Name          types.String `tfsdk:"name"`
	TaggedPorts   types.Set    `tfsdk:"tagged_ports"`
	UntaggedPorts types.Set    `tfsdk:"untagged_ports"`
	Routing       types.Bool   `tfsdk:"routing"`
}

type vlanDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	VlanID        types.Int64  `tfsdk:"vlan_id"`
	Name          types.String `tfsdk:"name"`
	TaggedPorts   types.Set    `tfsdk:"tagged_ports"`
	UntaggedPorts types.Set    `tfsdk:"untagged_ports"`
	Routing       types.Bool   `tfsdk:"routing"`
}

func NewVlanDataSource() datasource.DataSource {
	return &vlanDataSource{}
}

// vlanEntryAttributes describes a VLAN read off the switch. Every attribute is
// computed; the singular data source replaces `vlan_id` with the lookup key.
func vlanEntryAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"vlan_id": schema.Int64Attribute{
			MarkdownDescription: "The 802.1Q VLAN id.",
			Computed:            true,
		},
		"name": schema.StringAttribute{
			MarkdownDescription: "VLAN name, empty when the switch has none for it.",
			Computed:            true,
		},
		"tagged_ports": schema.SetAttribute{
			MarkdownDescription: "Ports carrying this VLAN tagged, in FASTPATH notation such as `0/1`.",
			Computed:            true,
			ElementType:         types.StringType,
		},
		"untagged_ports": schema.SetAttribute{
			MarkdownDescription: "Ports participating in this VLAN untagged.",
			Computed:            true,
			ElementType:         types.StringType,
		},
		"routing": schema.BoolAttribute{
			MarkdownDescription: "Whether `vlan routing` is enabled for this VLAN.",
			Computed:            true,
		},
	}
}

func (d *vlanDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vlan"
}

func (d *vlanDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attributes := vlanEntryAttributes()
	attributes["id"] = schema.StringAttribute{
		MarkdownDescription: "The VLAN id as a string.",
		Computed:            true,
	}
	attributes["vlan_id"] = schema.Int64Attribute{
		MarkdownDescription: "The 802.1Q VLAN id to look up.",
		Required:            true,
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "One VLAN as the switch has it configured, read from `show running-config`.\n\n" +
			"Reading a VLAN the switch does not define is an error. Use `netgear_vlans` to " +
			"discover which ids exist.",
		Attributes: attributes,
	}
}

func (d *vlanDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.data = switchFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *vlanDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config vlanDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if d.data == nil {
		resp.Diagnostics.Append(errNotConfigured("netgear_vlan")...)
		return
	}

	running, err := d.data.runningConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read the Switch Configuration", err.Error())
		return
	}

	id := config.VlanID.ValueInt64()

	vlan, found := running.VLAN(id)
	if !found {
		resp.Diagnostics.AddError(
			"VLAN Not Found",
			"The switch has no VLAN "+itoa(id)+". Read netgear_vlans to see which ids it defines.",
		)
		return
	}

	entry := vlanEntry(vlan)

	resp.Diagnostics.Append(resp.State.Set(ctx, &vlanDataSourceModel{
		ID:            types.StringValue(itoa(vlan.ID)),
		VlanID:        entry.VlanID,
		Name:          entry.Name,
		TaggedPorts:   entry.TaggedPorts,
		UntaggedPorts: entry.UntaggedPorts,
		Routing:       entry.Routing,
	})...)
}

// vlanEntry maps a parsed VLAN onto the model both VLAN data sources report.
func vlanEntry(vlan *fastpath.VLAN) vlanEntryModel {
	return vlanEntryModel{
		VlanID:        types.Int64Value(vlan.ID),
		Name:          types.StringValue(vlan.Name),
		TaggedPorts:   stringSet(vlan.Tagged),
		UntaggedPorts: stringSet(vlan.Untagged),
		Routing:       types.BoolValue(vlan.Routing),
	}
}
