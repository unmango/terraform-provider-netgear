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
	_ datasource.DataSource              = &lagsDataSource{}
	_ datasource.DataSourceWithConfigure = &lagsDataSource{}
)

type lagsDataSource struct {
	data *switchData
}

type lagsDataSourceModel struct {
	ID   types.String    `tfsdk:"id"`
	Lags []lagEntryModel `tfsdk:"lags"`
}

func NewLagsDataSource() datasource.DataSource {
	return &lagsDataSource{}
}

func (d *lagsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lags"
}

func (d *lagsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The link aggregation groups the switch has configured, read from " +
			"`show running-config`.\n\n" +
			"The switch defines every LAG interface whether or not it is used, so a group left " +
			"entirely at its defaults is absent here rather than listed empty.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Constant `lags`. The framework requires an id.",
				Computed:            true,
			},
			"lags": schema.ListNestedAttribute{
				MarkdownDescription: "The groups, in ascending id order.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: lagEntryAttributes(),
				},
			},
		},
	}
}

func (d *lagsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.data = switchFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *lagsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.data == nil {
		resp.Diagnostics.Append(errNotConfigured("netgear_lags")...)
		return
	}

	running, err := d.data.runningConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read the Switch Configuration", err.Error())
		return
	}

	state := lagsDataSourceModel{
		ID:   types.StringValue("lags"),
		Lags: []lagEntryModel{},
	}
	for _, id := range slices.Sorted(maps.Keys(running.LAGs)) {
		state.Lags = append(state.Lags, lagEntry(running.LAGs[id]))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
