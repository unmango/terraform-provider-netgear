package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	data *switchData
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
	r.data = switchFromProviderData(req.ProviderData, &resp.Diagnostics)
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

	if r.data == nil {
		resp.Diagnostics.Append(errNotConfigured("netgear_vlan")...)
		return
	}

	members, ok := vlanMembers(ctx, plan, &resp.Diagnostics)
	if !ok {
		return
	}

	id := plan.VlanID.ValueInt64()

	// The VLAN database is a privileged mode context, not part of configure.
	cmds := []string{"vlan database", "vlan " + itoa(id)}
	if !plan.Name.IsNull() {
		cmds = append(cmds, "vlan name "+itoa(id)+" "+quote(plan.Name.ValueString()))
	}
	if plan.Routing.ValueBool() {
		cmds = append(cmds, "vlan routing "+itoa(id))
	}
	cmds = append(cmds, "exit")

	if len(members.member) > 0 {
		cmds = append(cmds, "configure")
		for _, port := range members.ports() {
			cmds = append(cmds, "interface "+port, "vlan participation include "+itoa(id))
			if members.tagged[port] {
				cmds = append(cmds, "vlan tagging "+itoa(id))
			}
			cmds = append(cmds, "exit")
		}
		cmds = append(cmds, "exit")
	}

	if _, err := r.data.apply(ctx, cmds...); err != nil {
		resp.Diagnostics.AddError("Unable to Create the VLAN", err.Error())
		return
	}

	plan.ID = types.StringValue(itoa(id))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vlanResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vlanResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.data == nil {
		resp.Diagnostics.Append(errNotConfigured("netgear_vlan")...)
		return
	}

	config, err := r.data.runningConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read the Switch Configuration", err.Error())
		return
	}

	vlan, found := config.VLAN(state.VlanID.ValueInt64())
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(itoa(vlan.ID))
	state.Name = stringOrNull(state.Name, vlan.Name)
	state.Routing = types.BoolValue(vlan.Routing)
	state.TaggedPorts = setOrNull(state.TaggedPorts, vlan.Tagged)
	state.UntaggedPorts = setOrNull(state.UntaggedPorts, vlan.Untagged)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *vlanResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state vlanResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.data == nil {
		resp.Diagnostics.Append(errNotConfigured("netgear_vlan")...)
		return
	}

	planned, ok := vlanMembers(ctx, plan, &resp.Diagnostics)
	if !ok {
		return
	}
	current, ok := vlanMembers(ctx, state, &resp.Diagnostics)
	if !ok {
		return
	}

	id := plan.VlanID.ValueInt64()

	// Database settings and port membership live in different modes, so they are
	// gathered separately and each block is only entered when it has work.
	var database []string
	if !plan.Name.Equal(state.Name) {
		if plan.Name.IsNull() {
			database = append(database, "no vlan name "+itoa(id))
		} else {
			database = append(database, "vlan name "+itoa(id)+" "+quote(plan.Name.ValueString()))
		}
	}
	if !plan.Routing.Equal(state.Routing) {
		if plan.Routing.ValueBool() {
			database = append(database, "vlan routing "+itoa(id))
		} else {
			database = append(database, "no vlan routing "+itoa(id))
		}
	}

	var cmds []string
	if len(database) > 0 {
		cmds = append(cmds, "vlan database")
		cmds = append(cmds, database...)
		cmds = append(cmds, "exit")
	}

	if membership := membershipCommands(id, current, planned); len(membership) > 0 {
		cmds = append(cmds, "configure")
		cmds = append(cmds, membership...)
		cmds = append(cmds, "exit")
	}

	if len(cmds) > 0 {
		if _, err := r.data.apply(ctx, cmds...); err != nil {
			resp.Diagnostics.AddError("Unable to Update the VLAN", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(itoa(id))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
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

	if r.data == nil {
		resp.Diagnostics.Append(errNotConfigured("netgear_vlan")...)
		return
	}

	id := state.VlanID.ValueInt64()

	if _, err := r.data.apply(ctx,
		"vlan database",
		"no vlan "+itoa(id),
		"exit",
	); err != nil {
		resp.Diagnostics.AddError("Unable to Delete the VLAN", err.Error())
	}
}

// vlanMembership is the membership of one VLAN, keyed by port.
type vlanMembership struct {
	member map[string]struct{}
	tagged map[string]bool
}

// ports returns the member ports in a stable order.
func (m vlanMembership) ports() []string {
	return sortedKeys(m.member)
}

// vlanMembers reads the two membership sets off a model into one lookup.
func vlanMembers(ctx context.Context, model vlanResourceModel, diags *diag.Diagnostics) (vlanMembership, bool) {
	members := vlanMembership{
		member: map[string]struct{}{},
		tagged: map[string]bool{},
	}

	tagged, ok := portSet(ctx, model.TaggedPorts, diags)
	if !ok {
		return members, false
	}
	untagged, ok := portSet(ctx, model.UntaggedPorts, diags)
	if !ok {
		return members, false
	}

	for port := range tagged {
		members.member[port] = struct{}{}
		members.tagged[port] = true
	}
	for port := range untagged {
		members.member[port] = struct{}{}
	}

	return members, true
}

// membershipCommands moves the switch from current membership to planned, leaving
// ports that are already correct untouched.
func membershipCommands(id int64, current, planned vlanMembership) []string {
	var cmds []string

	for _, port := range planned.ports() {
		_, joined := current.member[port]

		switch {
		case !joined:
			cmds = append(cmds, "interface "+port, "vlan participation include "+itoa(id))
			if planned.tagged[port] {
				cmds = append(cmds, "vlan tagging "+itoa(id))
			}
			cmds = append(cmds, "exit")
		case planned.tagged[port] && !current.tagged[port]:
			cmds = append(cmds, "interface "+port, "vlan tagging "+itoa(id), "exit")
		case !planned.tagged[port] && current.tagged[port]:
			cmds = append(cmds, "interface "+port, "no vlan tagging "+itoa(id), "exit")
		}
	}

	for _, port := range current.ports() {
		if _, kept := planned.member[port]; kept {
			continue
		}
		cmds = append(cmds, "interface "+port, "vlan participation exclude "+itoa(id), "exit")
	}

	return cmds
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
