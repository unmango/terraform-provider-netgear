package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/UnstoppableMango/terraform-provider-netgear/internal/fastpath"
)

var (
	_ datasource.DataSource              = &interfaceDataSource{}
	_ datasource.DataSourceWithConfigure = &interfaceDataSource{}
)

type interfaceDataSource struct {
	data *switchData
}

// interfaceEntryModel is one port as the switch reports it, shared by the
// singular and the plural data source.
type interfaceEntryModel struct {
	Port        types.String `tfsdk:"port"`
	Description types.String `tfsdk:"description"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	Speed       types.String `tfsdk:"speed"`
	PVID        types.Int64  `tfsdk:"pvid"`
	MTU         types.Int64  `tfsdk:"mtu"`
	Vlans       types.Set    `tfsdk:"vlans"`
	TaggedVlans types.Set    `tfsdk:"tagged_vlans"`
	Lag         types.String `tfsdk:"lag"`
	AdminStatus types.String `tfsdk:"admin_status"`
	LinkStatus  types.String `tfsdk:"link_status"`
}

type interfaceDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Port        types.String `tfsdk:"port"`
	Description types.String `tfsdk:"description"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	Speed       types.String `tfsdk:"speed"`
	PVID        types.Int64  `tfsdk:"pvid"`
	MTU         types.Int64  `tfsdk:"mtu"`
	Vlans       types.Set    `tfsdk:"vlans"`
	TaggedVlans types.Set    `tfsdk:"tagged_vlans"`
	Lag         types.String `tfsdk:"lag"`
	AdminStatus types.String `tfsdk:"admin_status"`
	LinkStatus  types.String `tfsdk:"link_status"`
}

func NewInterfaceDataSource() datasource.DataSource {
	return &interfaceDataSource{}
}

// interfaceEntryAttributes describes a port read off the switch. Every attribute
// is computed; the singular data source replaces `port` with the lookup key.
func interfaceEntryAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"port": schema.StringAttribute{
			MarkdownDescription: "Port id in FASTPATH notation, such as `0/1`.",
			Computed:            true,
		},
		"description": schema.StringAttribute{
			MarkdownDescription: "Port description, empty when the switch has none for it.",
			Computed:            true,
		},
		"enabled": schema.BoolAttribute{
			MarkdownDescription: "Whether the port is administratively up. `false` means the config carries `shutdown`.",
			Computed:            true,
		},
		"speed": schema.StringAttribute{
			MarkdownDescription: "Speed and duplex setting, empty when the port is left at the switch default.",
			Computed:            true,
		},
		"pvid": schema.Int64Attribute{
			MarkdownDescription: "VLAN untagged ingress traffic is assigned to. Zero when the port is left on the default VLAN.",
			Computed:            true,
		},
		"mtu": schema.Int64Attribute{
			MarkdownDescription: "Maximum frame size in bytes. Zero when the port is left at the switch default.",
			Computed:            true,
		},
		"vlans": schema.SetAttribute{
			MarkdownDescription: "VLANs the port participates in.",
			Computed:            true,
			ElementType:         types.Int64Type,
		},
		"tagged_vlans": schema.SetAttribute{
			MarkdownDescription: "The subset of `vlans` the port carries tagged.",
			Computed:            true,
			ElementType:         types.Int64Type,
		},
		"lag": schema.StringAttribute{
			MarkdownDescription: "Interface id of the LAG the port belongs to, such as `3/1`. Empty when the port stands alone.",
			Computed:            true,
		},
		"admin_status": schema.StringAttribute{
			MarkdownDescription: "Administrative status reported by `show port`.",
			Computed:            true,
		},
		"link_status": schema.StringAttribute{
			MarkdownDescription: "Observed link state, `up` or `down`.",
			Computed:            true,
		},
	}
}

func (d *interfaceDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_interface"
}

func (d *interfaceDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attributes := interfaceEntryAttributes()
	attributes["id"] = schema.StringAttribute{
		MarkdownDescription: "Same value as `port`.",
		Computed:            true,
	}
	attributes["port"] = schema.StringAttribute{
		MarkdownDescription: "Port id to look up, in either FASTPATH spelling: `0/1` or `g1`.",
		Required:            true,
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "One switch port, its configuration from `show running-config` and its live " +
			"state from `show port`.\n\n" +
			"Ports left entirely at their defaults do not appear in the running config, so the " +
			"configuration attributes are empty for them while the status attributes are still " +
			"reported.",
		Attributes: attributes,
	}
}

func (d *interfaceDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.data = switchFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *interfaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config interfaceDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if d.data == nil {
		resp.Diagnostics.Append(errNotConfigured("netgear_interface")...)
		return
	}

	running, err := d.data.runningConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read the Switch Configuration", err.Error())
		return
	}

	port := config.Port.ValueString()

	status, err := d.data.readStatus(ctx, port)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read the Port Status", err.Error())
		return
	}

	iface, configured := running.Interface(port)
	if !configured && !status.Known() {
		resp.Diagnostics.AddError(
			"Port Not Found",
			"The switch reported nothing for port "+port+". Read netgear_interfaces to see which ports it has.",
		)
		return
	}
	if iface == nil {
		iface = &fastpath.Interface{Port: fastpath.NormalizePort(port)}
	}

	entry := interfaceEntry(iface, status)

	resp.Diagnostics.Append(resp.State.Set(ctx, &interfaceDataSourceModel{
		ID:          entry.Port,
		Port:        entry.Port,
		Description: entry.Description,
		Enabled:     entry.Enabled,
		Speed:       entry.Speed,
		PVID:        entry.PVID,
		MTU:         entry.MTU,
		Vlans:       entry.Vlans,
		TaggedVlans: entry.TaggedVlans,
		Lag:         entry.Lag,
		AdminStatus: entry.AdminStatus,
		LinkStatus:  entry.LinkStatus,
	})...)
}

// interfaceEntry maps a parsed port and its status onto the model both interface
// data sources report.
func interfaceEntry(iface *fastpath.Interface, status fastpath.PortStatus) interfaceEntryModel {
	return interfaceEntryModel{
		Port:        types.StringValue(iface.Port),
		Description: types.StringValue(iface.Description),
		Enabled:     types.BoolValue(!iface.Shutdown),
		Speed:       types.StringValue(iface.Speed),
		PVID:        types.Int64Value(iface.PVID),
		MTU:         types.Int64Value(iface.MTU),
		Vlans:       int64Set(iface.Participation),
		TaggedVlans: int64Set(iface.Tagging),
		Lag:         types.StringValue(iface.LAG),
		AdminStatus: types.StringValue(status.AdminStatus),
		LinkStatus:  types.StringValue(status.LinkStatus),
	}
}
