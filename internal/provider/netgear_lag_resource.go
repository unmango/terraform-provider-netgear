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
			"Groups are not created or destroyed: the switch defines every LAG interface whether " +
			"or not it is used, so this resource adopts one. Destroying it releases the members and " +
			"returns the settings to their defaults rather than removing anything.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The LAG id as a string. Used as the import id.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"lag_id": schema.Int64Attribute{
				MarkdownDescription: "LAG number. The GS724Tv4 defines 1 through 26. Changing this targets a different group, so it replaces the resource.",
				Required:            true,
				Validators: []validator.Int64{
					int64validator.Between(1, 26),
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
				MarkdownDescription: "`lacp` for 802.3ad negotiation, or `static` for an unconditional bundle. " +
					"The switch's own default is `static`, so a group left at the default `lacp` is recorded " +
					"as `no port-channel static` in the running config.",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(lagModeLACP),
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
				MarkdownDescription: "Load balance selector, passed to `port-channel load-balance`. Leave unset to keep the switch default.",
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
	cmds = append(cmds, modeCommand(plan.Mode.ValueString()))
	if !plan.HashMode.IsNull() {
		cmds = append(cmds, "port-channel load-balance "+itoa(plan.HashMode.ValueInt64()))
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
		settings = append(settings, modeCommand(plan.Mode.ValueString()))
	}
	if !plan.HashMode.Equal(state.HashMode) {
		if plan.HashMode.IsNull() {
			settings = append(settings, "no port-channel load-balance")
		} else {
			settings = append(settings, "port-channel load-balance "+itoa(plan.HashMode.ValueInt64()))
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

	// The group itself cannot be removed: the switch defines every LAG interface
	// whether or not it is used. Releasing the members and returning the settings
	// to their defaults is what removes it from the running config.
	cmds := []string{"configure"}
	for _, port := range sortedKeys(members) {
		cmds = append(cmds, "interface "+port, "deleteport "+interfaceID, "exit")
	}

	var reset []string
	if !state.Name.IsNull() {
		reset = append(reset, "no description")
	}
	if state.Mode.ValueString() != lagModeStatic {
		reset = append(reset, modeCommand(lagModeStatic))
	}
	if !state.HashMode.IsNull() {
		reset = append(reset, "no port-channel load-balance")
	}
	if !state.Enabled.ValueBool() {
		reset = append(reset, "no shutdown")
	}

	if len(reset) > 0 {
		cmds = append(cmds, "interface lag "+itoa(id))
		cmds = append(cmds, reset...)
		cmds = append(cmds, "exit")
	}
	cmds = append(cmds, "exit")

	if _, err := r.data.apply(ctx, cmds...); err != nil {
		resp.Diagnostics.AddError("Unable to Reset the LAG", err.Error())
	}
}

// modeCommand selects between an unconditional bundle and LACP negotiation.
// Static is the switch's own default, so LACP is the setting it records.
func modeCommand(mode string) string {
	if mode == lagModeStatic {
		return "port-channel static"
	}

	return "no port-channel static"
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
