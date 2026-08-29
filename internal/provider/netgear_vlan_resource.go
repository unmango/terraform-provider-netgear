package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = &vlanResource{}
	_ resource.ResourceWithConfigure      = &vlanResource{}
	_ resource.ResourceWithImportState    = &vlanResource{}
	_ resource.ResourceWithValidateConfig = &vlanResource{}
)

type vlanResource struct {
	client Client
}

type vlanResourceModel struct {
	ID            types.String `tfsdk:"id"`
	VlanID        types.Int64  `tfsdk:"vlan_id"`
	Name          types.String `tfsdk:"name"`
	TaggedPorts   types.Set    `tfsdk:"tagged_ports"`
	UntaggedPorts types.Set    `tfsdk:"untagged_ports"`
	Routing       types.Bool   `tfsdk:"routing"`
}

func NewVlanResource() resource.Resource {
	return &vlanResource{}
}

func (r *vlanResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vlan"
}

func (r *vlanResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A VLAN and its port membership, as configured through the FASTPATH `vlan database` " +
			"and per interface `vlan participation` commands.\n\n" +
			"Membership is owned by this resource. A port listed here should not also have its VLAN " +
			"settings managed by `netgear_interface`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The VLAN id as a string. Used as the import id.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"vlan_id": schema.Int64Attribute{
				MarkdownDescription: "The 802.1Q VLAN id. Changing this replaces the VLAN.",
				Required:            true,
				Validators: []validator.Int64{
					int64validator.Between(1, 4093),
				},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "VLAN name. FASTPATH accepts up to 32 characters.",
				Optional:            true,
			},
			"tagged_ports": schema.SetAttribute{
				MarkdownDescription: "Ports carrying this VLAN tagged, in FASTPATH notation such as `0/1`.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"untagged_ports": schema.SetAttribute{
				MarkdownDescription: "Ports participating in this VLAN untagged. Set the port's `pvid` " +
					"through `netgear_interface` if ingress traffic should also land in this VLAN.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"routing": schema.BoolAttribute{
				MarkdownDescription: "Enable `vlan routing` for this VLAN. Only present on L3 capable firmware.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
		},
	}
}

func (r *vlanResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

// ValidateConfig rejects a port that appears in both membership sets, which
// FASTPATH cannot express: a port is either tagged or untagged in a given VLAN.
func (r *vlanResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config vlanResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tagged, ok := portSet(ctx, config.TaggedPorts, &resp.Diagnostics)
	if !ok {
		return
	}
	untagged, ok := portSet(ctx, config.UntaggedPorts, &resp.Diagnostics)
	if !ok {
		return
	}

	for port := range untagged {
		if _, both := tagged[port]; both {
			resp.Diagnostics.AddAttributeError(
				path.Root("untagged_ports"),
				"Port Listed Twice",
				"Port "+port+" appears in both tagged_ports and untagged_ports. A port can only be one or the other in a VLAN.",
			)
		}
	}
}

func (r *vlanResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vlanResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// configure
	// vlan database
	// vlan <vlan_id>
	// vlan name <vlan_id> "<name>"
	// exit
	// then per member port:
	// interface <port>
	// vlan participation include <vlan_id>
	// vlan tagging <vlan_id>          (tagged members only)
	resp.Diagnostics.Append(notImplemented("netgear_vlan", "Create")...)
}

func (r *vlanResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vlanResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read `show running-config` once and parse the vlan database block plus each
	// interface's `vlan participation` and `vlan tagging` lines. A VLAN that is no
	// longer present calls resp.State.RemoveResource(ctx) and returns.
	resp.Diagnostics.Append(notImplemented("netgear_vlan", "Read")...)
}

func (r *vlanResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state vlanResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Diff the membership sets against state:
	// removed members:            interface <port>; vlan participation exclude <vlan_id>
	// tagged becoming untagged:   interface <port>; no vlan tagging <vlan_id>
	// renames:                    vlan database; vlan name <vlan_id> "<name>"
	resp.Diagnostics.Append(notImplemented("netgear_vlan", "Update")...)
}

func (r *vlanResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vlanResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.VlanID.ValueInt64() == 1 {
		resp.Diagnostics.AddError(
			"Cannot Delete the Default VLAN",
			"VLAN 1 is built into FASTPATH and cannot be removed. Remove it from Terraform state instead.",
		)
		return
	}

	// configure
	// vlan database
	// no vlan <vlan_id>
	resp.Diagnostics.Append(notImplemented("netgear_vlan", "Delete")...)
}

func (r *vlanResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Import Id",
			"Expected a VLAN id such as \"10\", got "+req.ID+".",
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("vlan_id"), id)...)
}
