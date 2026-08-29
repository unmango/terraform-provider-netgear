package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &lagResource{}
	_ resource.ResourceWithConfigure   = &lagResource{}
	_ resource.ResourceWithImportState = &lagResource{}
)

type lagResource struct {
	data *switchData
}

type lagResourceModel struct {
	ID          types.String `tfsdk:"id"`
	LagID       types.Int64  `tfsdk:"lag_id"`
	Name        types.String `tfsdk:"name"`
	Mode        types.String `tfsdk:"mode"`
	Members     types.Set    `tfsdk:"members"`
	HashMode    types.Int64  `tfsdk:"hash_mode"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	InterfaceID types.String `tfsdk:"interface_id"`
}

func NewLagResource() resource.Resource {
	return &lagResource{}
}

func (r *lagResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lag"
}

func (r *lagResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A link aggregation group.\n\n" +
			"FASTPATH moves VLAN configuration from member ports onto the LAG interface, so a port " +
			"listed in `members` should not have its VLAN settings managed elsewhere. Reference " +
			"`interface_id` from a `netgear_vlan` to put the LAG in a VLAN.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The LAG id as a string. Used as the import id.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"lag_id": schema.Int64Attribute{
				MarkdownDescription: "LAG number. The GS724Tv4 supports 1 through 8. Changing this replaces the LAG.",
				Required:            true,
				Validators: []validator.Int64{
					int64validator.Between(1, 8),
				},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "LAG name.",
				Optional:            true,
			},
			"mode": schema.StringAttribute{
				MarkdownDescription: "`lacp` for 802.3ad negotiation, or `static` for an unconditional bundle.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("lacp"),
				Validators: []validator.String{
					stringvalidator.OneOf("lacp", "static"),
				},
			},
			"members": schema.SetAttribute{
				MarkdownDescription: "Physical ports in the bundle, in FASTPATH notation such as `0/1`.",
				Required:            true,
				ElementType:         types.StringType,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
			},
			"hash_mode": schema.Int64Attribute{
				MarkdownDescription: "FASTPATH load balance selector, chosen from the values the firmware accepts for `hashing-mode`.",
				Optional:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the LAG is administratively up.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"interface_id": schema.StringAttribute{
				MarkdownDescription: "The LAG's interface id, such as `3/1`. Use this where other resources expect a port.",
				Computed:            true,
			},
		},
	}
}

func (r *lagResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.data = switchFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *lagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan lagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// configure
	// interface lag <lag_id>
	// description "<name>"
	// staticcapability                 (static mode only)
	// hashing-mode <hash_mode>
	// no shutdown | shutdown
	// exit
	// then per member:
	// interface <port>
	// addport <interface_id>
	resp.Diagnostics.Append(notImplemented("netgear_lag", "Create")...)
}

func (r *lagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state lagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse `show port-channel <interface_id>` for members and mode, falling back
	// to the running config. A missing LAG calls resp.State.RemoveResource(ctx).
	resp.Diagnostics.Append(notImplemented("netgear_lag", "Read")...)
}

func (r *lagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state lagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Diff members against state: `deleteport <interface_id>` for removals,
	// `addport <interface_id>` for additions, both from the member port's own
	// interface context. Mode changes toggle `staticcapability`.
	resp.Diagnostics.Append(notImplemented("netgear_lag", "Update")...)
}

func (r *lagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state lagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Remove every member with `deleteport`, then `no interface lag <lag_id>`.
	resp.Diagnostics.Append(notImplemented("netgear_lag", "Delete")...)
}

func (r *lagResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Import Id",
			"Expected a LAG id such as \"1\", got "+req.ID+".",
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("lag_id"), id)...)
}
