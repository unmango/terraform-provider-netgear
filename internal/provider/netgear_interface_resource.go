package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &interfaceResource{}
	_ resource.ResourceWithConfigure   = &interfaceResource{}
	_ resource.ResourceWithImportState = &interfaceResource{}
)

type interfaceResource struct {
	data *switchData
}

type interfaceResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Port        types.String `tfsdk:"port"`
	Description types.String `tfsdk:"description"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	Speed       types.String `tfsdk:"speed"`
	PVID        types.Int64  `tfsdk:"pvid"`
	MTU         types.Int64  `tfsdk:"mtu"`
	FlowControl types.Bool   `tfsdk:"flow_control"`
	AdminStatus types.String `tfsdk:"admin_status"`
	LinkStatus  types.String `tfsdk:"link_status"`
}

func NewInterfaceResource() resource.Resource {
	return &interfaceResource{}
}

func (r *interfaceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_interface"
}

func (r *interfaceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Configuration of a single physical switch port.\n\n" +
			"Ports are not created or destroyed, so this resource adopts an existing port. " +
			"Destroying it returns the managed attributes to their FASTPATH defaults rather than " +
			"removing anything.\n\n" +
			"VLAN membership belongs to `netgear_vlan`. This resource sets only `pvid`, the VLAN " +
			"untagged ingress traffic lands in, so a port should not be listed in a VLAN's " +
			"`untagged_ports` and given a conflicting `pvid` here.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Same value as `port`. Used as the import id.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"port": schema.StringAttribute{
				MarkdownDescription: "Port id in FASTPATH notation, such as `0/1`. Changing this targets a different port, so it replaces the resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Port description.",
				Optional:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the port is administratively up. `false` issues `shutdown`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"speed": schema.StringAttribute{
				MarkdownDescription: "Speed and duplex setting.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("auto", "10-half", "10-full", "100-half", "100-full", "1000-full"),
				},
			},
			"pvid": schema.Int64Attribute{
				MarkdownDescription: "VLAN untagged ingress traffic is assigned to. Defaults to VLAN 1 on the switch.",
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.Between(1, 4093),
				},
			},
			"mtu": schema.Int64Attribute{
				MarkdownDescription: "Maximum frame size in bytes.",
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.Between(1518, 9216),
				},
			},
			"flow_control": schema.BoolAttribute{
				MarkdownDescription: "802.3x flow control. Availability is firmware dependent.",
				Optional:            true,
			},
			"admin_status": schema.StringAttribute{
				MarkdownDescription: "Administrative status reported by the switch.",
				Computed:            true,
			},
			"link_status": schema.StringAttribute{
				MarkdownDescription: "Observed link state, `up` or `down`.",
				Computed:            true,
			},
		},
	}
}

func (r *interfaceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.data = switchFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *interfaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan interfaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// configure
	// interface <port>
	// description "<description>"
	// no shutdown | shutdown
	// speed <speed> | auto negotiate
	// vlan pvid <pvid>
	// mtu <mtu>
	// Then read admin_status and link_status back from `show interfaces status <port>`.
	resp.Diagnostics.Append(notImplemented("netgear_interface", "Create")...)
}

func (r *interfaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state interfaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse the `interface <port>` block out of `show running-config` for the
	// configured attributes, and `show interfaces status <port>` for the computed
	// ones. A port the switch does not report calls resp.State.RemoveResource(ctx).
	resp.Diagnostics.Append(notImplemented("netgear_interface", "Read")...)
}

func (r *interfaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state interfaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Emit only the changed settings. An attribute cleared in configuration is
	// reset with its `no` form, for example `no description`.
	resp.Diagnostics.Append(notImplemented("netgear_interface", "Update")...)
}

func (r *interfaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state interfaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The port survives, so reset what was managed:
	// interface <port>
	// no description
	// no shutdown
	// vlan pvid 1
	resp.Diagnostics.Append(notImplemented("netgear_interface", "Delete")...)
}

func (r *interfaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("port"), req.ID)...)
}
