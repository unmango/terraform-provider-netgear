package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/unmango/terraform-provider-netgear/internal/fastpath"
)

var (
	_ datasource.DataSource              = &lagDataSource{}
	_ datasource.DataSourceWithConfigure = &lagDataSource{}
)

type lagDataSource struct {
	data *switchData
}

// lagEntryModel is one link aggregation group as the switch reports it, shared by
// the singular and the plural data source.
type lagEntryModel struct {
	LagID       types.Int64  `tfsdk:"lag_id"`
	Name        types.String `tfsdk:"name"`
	Mode        types.String `tfsdk:"mode"`
	Members     types.Set    `tfsdk:"members"`
	HashMode    types.Int64  `tfsdk:"hash_mode"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	InterfaceID types.String `tfsdk:"interface_id"`
}

type lagDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	LagID       types.Int64  `tfsdk:"lag_id"`
	Name        types.String `tfsdk:"name"`
	Mode        types.String `tfsdk:"mode"`
	Members     types.Set    `tfsdk:"members"`
	HashMode    types.Int64  `tfsdk:"hash_mode"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	InterfaceID types.String `tfsdk:"interface_id"`
}

func NewLagDataSource() datasource.DataSource {
	return &lagDataSource{}
}

// lagEntryAttributes describes a group read off the switch. Every attribute is
// computed; the singular data source replaces `lag_id` with the lookup key.
func lagEntryAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"lag_id": schema.Int64Attribute{
			MarkdownDescription: "LAG number.",
			Computed:            true,
		},
		"name": schema.StringAttribute{
			MarkdownDescription: "LAG name, empty when the switch has none for it.",
			Computed:            true,
		},
		"mode": schema.StringAttribute{
			MarkdownDescription: "`lacp` for 802.3ad negotiation, or `static` for an unconditional bundle.",
			Computed:            true,
		},
		"members": schema.SetAttribute{
			MarkdownDescription: "Physical ports in the bundle, in FASTPATH notation such as `0/1`.",
			Computed:            true,
			ElementType:         types.StringType,
		},
		"hash_mode": schema.Int64Attribute{
			MarkdownDescription: "Load balance selector. Zero when the group is left at the switch default.",
			Computed:            true,
		},
		"enabled": schema.BoolAttribute{
			MarkdownDescription: "Whether the LAG is administratively up.",
			Computed:            true,
		},
		"interface_id": schema.StringAttribute{
			MarkdownDescription: "The LAG's interface id, such as `3/1`. Use this where other resources expect a port.",
			Computed:            true,
		},
	}
}

func (d *lagDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lag"
}

func (d *lagDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attributes := lagEntryAttributes()
	attributes["id"] = schema.StringAttribute{
		MarkdownDescription: "The LAG id as a string.",
		Computed:            true,
	}
	attributes["lag_id"] = schema.Int64Attribute{
		MarkdownDescription: "LAG number to look up. The GS724Tv4 defines 1 through 26.",
		Required:            true,
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "One link aggregation group, read from `show running-config`.\n\n" +
			"The switch defines every LAG interface whether or not it is used, but only groups " +
			"that carry configuration appear in the running config. Reading one that does not is " +
			"an error.",
		Attributes: attributes,
	}
}

func (d *lagDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.data = switchFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *lagDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config lagDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if d.data == nil {
		resp.Diagnostics.Append(errNotConfigured("netgear_lag")...)
		return
	}

	running, err := d.data.runningConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read the Switch Configuration", err.Error())
		return
	}

	id := config.LagID.ValueInt64()

	lag, found := running.LAG(id)
	if !found {
		resp.Diagnostics.AddError(
			"LAG Not Found",
			"The running config carries nothing for LAG "+itoa(id)+", which means the group is "+
				"entirely at its defaults. Read netgear_lags to see which groups are configured.",
		)
		return
	}

	entry := lagEntry(lag)

	resp.Diagnostics.Append(resp.State.Set(ctx, &lagDataSourceModel{
		ID:          types.StringValue(itoa(lag.ID)),
		LagID:       entry.LagID,
		Name:        entry.Name,
		Mode:        entry.Mode,
		Members:     entry.Members,
		HashMode:    entry.HashMode,
		Enabled:     entry.Enabled,
		InterfaceID: entry.InterfaceID,
	})...)
}

// lagEntry maps a parsed group onto the model both LAG data sources report.
func lagEntry(lag *fastpath.LAG) lagEntryModel {
	return lagEntryModel{
		LagID:       types.Int64Value(lag.ID),
		Name:        types.StringValue(lag.Name),
		Mode:        types.StringValue(lag.Mode),
		Members:     stringSet(lag.Members),
		HashMode:    types.Int64Value(lag.HashMode),
		Enabled:     types.BoolValue(!lag.Shutdown),
		InterfaceID: types.StringValue(lag.InterfaceID),
	}
}
