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

const (
	lagModeLACP   = "lacp"
	lagModeStatic = "static"
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
			"`interface_id` from a `netgear_vlan` to put the LAG in a VLAN.\n\n" +
			"~> The CLI is undocumented on smart switches. This resource uses the `interface lag <id>` " +
			"form with `addport` and `deleteport`, and `staticcapability` for static mode. Firmware " +
			"that spells these differently rejects the commands with an invalid input error.",
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
				Default:             stringdefault.StaticString(lagModeLACP),
				Validators: []validator.String{
					stringvalidator.OneOf(lagModeLACP, lagModeStatic),
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

	if r.data == nil {
		resp.Diagnostics.Append(errNotConfigured("netgear_lag")...)
		return
	}

	members, ok := portSet(ctx, plan.Members, &resp.Diagnostics)
	if !ok {
		return
	}

	id := plan.LagID.ValueInt64()
	interfaceID := lagInterfaceID(id)

	cmds := []string{"configure", "interface lag " + itoa(id)}
	if !plan.Name.IsNull() {
		cmds = append(cmds, "description "+quote(plan.Name.ValueString()))
	}
	if plan.Mode.ValueString() == lagModeStatic {
		cmds = append(cmds, "staticcapability")
	}
	if !plan.HashMode.IsNull() {
		cmds = append(cmds, "hashing-mode "+itoa(plan.HashMode.ValueInt64()))
	}
	if plan.Enabled.ValueBool() {
		cmds = append(cmds, "no shutdown")
	} else {
		cmds = append(cmds, "shutdown")
	}
	cmds = append(cmds, "exit")

	for _, port := range sortedKeys(members) {
		cmds = append(cmds, "interface "+port, "addport "+interfaceID, "exit")
	}
	cmds = append(cmds, "exit")

	if _, err := r.data.apply(ctx, cmds...); err != nil {
		resp.Diagnostics.AddError("Unable to Create the LAG", err.Error())
		return
	}

	plan.ID = types.StringValue(itoa(id))
	plan.InterfaceID = types.StringValue(interfaceID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *lagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state lagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.data == nil {
		resp.Diagnostics.Append(errNotConfigured("netgear_lag")...)
		return
	}

	config, err := r.data.runningConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read the Switch Configuration", err.Error())
		return
	}

	lag, found := config.LAG(state.LagID.ValueInt64())
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(itoa(lag.ID))
	state.InterfaceID = types.StringValue(lag.InterfaceID)
	state.Name = stringOrNull(state.Name, lag.Name)
	state.Mode = types.StringValue(lag.Mode)
	state.Enabled = types.BoolValue(!lag.Shutdown)
	state.HashMode = int64OrNull(state.HashMode, lag.HashMode)
	state.Members = stringSet(lag.Members)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *lagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state lagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.data == nil {
		resp.Diagnostics.Append(errNotConfigured("netgear_lag")...)
		return
	}

	planned, ok := portSet(ctx, plan.Members, &resp.Diagnostics)
	if !ok {
		return
	}
	current, ok := portSet(ctx, state.Members, &resp.Diagnostics)
	if !ok {
		return
	}

	id := plan.LagID.ValueInt64()
	interfaceID := state.InterfaceID.ValueString()
	if interfaceID == "" {
		interfaceID = lagInterfaceID(id)
	}

	var settings []string
	if !plan.Name.Equal(state.Name) {
		if plan.Name.IsNull() {
			settings = append(settings, "no description")
		} else {
			settings = append(settings, "description "+quote(plan.Name.ValueString()))
		}
	}
	if !plan.Mode.Equal(state.Mode) {
		if plan.Mode.ValueString() == lagModeStatic {
			settings = append(settings, "staticcapability")
		} else {
			settings = append(settings, "no staticcapability")
		}
	}
	if !plan.HashMode.Equal(state.HashMode) {
		if plan.HashMode.IsNull() {
			settings = append(settings, "no hashing-mode")
		} else {
			settings = append(settings, "hashing-mode "+itoa(plan.HashMode.ValueInt64()))
		}
	}
	if !plan.Enabled.Equal(state.Enabled) {
		if plan.Enabled.ValueBool() {
			settings = append(settings, "no shutdown")
		} else {
			settings = append(settings, "shutdown")
		}
	}

	var cmds []string
	if len(settings) > 0 {
		cmds = append(cmds, "interface lag "+itoa(id))
		cmds = append(cmds, settings...)
		cmds = append(cmds, "exit")
	}

	// A port leaves the group from its own interface context, and so does a port
	// joining it, which is why membership is not part of the settings block.
	for _, port := range sortedKeys(current) {
		if _, kept := planned[port]; !kept {
			cmds = append(cmds, "interface "+port, "deleteport "+interfaceID, "exit")
		}
	}
	for _, port := range sortedKeys(planned) {
		if _, joined := current[port]; !joined {
			cmds = append(cmds, "interface "+port, "addport "+interfaceID, "exit")
		}
	}

	if len(cmds) > 0 {
		cmds = append([]string{"configure"}, cmds...)
		cmds = append(cmds, "exit")

		if _, err := r.data.apply(ctx, cmds...); err != nil {
			resp.Diagnostics.AddError("Unable to Update the LAG", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(itoa(id))
	plan.InterfaceID = types.StringValue(interfaceID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *lagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state lagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.data == nil {
		resp.Diagnostics.Append(errNotConfigured("netgear_lag")...)
		return
	}

	members, ok := portSet(ctx, state.Members, &resp.Diagnostics)
	if !ok {
		return
	}

	id := state.LagID.ValueInt64()
	interfaceID := state.InterfaceID.ValueString()
	if interfaceID == "" {
		interfaceID = lagInterfaceID(id)
	}

	cmds := []string{"configure"}
	for _, port := range sortedKeys(members) {
		cmds = append(cmds, "interface "+port, "deleteport "+interfaceID, "exit")
	}
	cmds = append(cmds, "no interface lag "+itoa(id), "exit")

	if _, err := r.data.apply(ctx, cmds...); err != nil {
		resp.Diagnostics.AddError("Unable to Delete the LAG", err.Error())
	}
}

// lagInterfaceID is the interface id FASTPATH gives a group, which other commands
// reference in place of a physical port.
func lagInterfaceID(id int64) string {
	return "3/" + itoa(id)
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
