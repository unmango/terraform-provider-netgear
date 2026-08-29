package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// defaultPVID is the VLAN a port falls back to when its pvid is cleared.
const defaultPVID = 1

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

	if r.data == nil {
		resp.Diagnostics.Append(errNotConfigured("netgear_interface")...)
		return
	}

	port := plan.Port.ValueString()

	cmds := []string{"configure", "interface " + port}
	if !plan.Description.IsNull() {
		cmds = append(cmds, "description "+quote(plan.Description.ValueString()))
	}
	if plan.Enabled.ValueBool() {
		cmds = append(cmds, "no shutdown")
	} else {
		cmds = append(cmds, "shutdown")
	}
	if !plan.Speed.IsNull() {
		cmds = append(cmds, speedCommand(plan.Speed.ValueString()))
	}
	if !plan.PVID.IsNull() {
		cmds = append(cmds, "vlan pvid "+itoa(plan.PVID.ValueInt64()))
	}
	if !plan.MTU.IsNull() {
		cmds = append(cmds, "mtu "+itoa(plan.MTU.ValueInt64()))
	}
	if !plan.FlowControl.IsNull() {
		cmds = append(cmds, flowControlCommand(plan.FlowControl.ValueBool()))
	}
	cmds = append(cmds, "exit", "exit")

	if _, err := r.data.apply(ctx, cmds...); err != nil {
		resp.Diagnostics.AddError("Unable to Configure the Port", err.Error())
		return
	}

	plan.ID = types.StringValue(port)
	r.applyStatus(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *interfaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state interfaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.data == nil {
		resp.Diagnostics.Append(errNotConfigured("netgear_interface")...)
		return
	}

	config, err := r.data.runningConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read the Switch Configuration", err.Error())
		return
	}

	port := state.Port.ValueString()

	iface, found := config.Interface(port)
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(port)
	state.Description = stringOrNull(state.Description, iface.Description)
	state.Enabled = types.BoolValue(!iface.Shutdown)
	state.Speed = stringOrNull(state.Speed, iface.Speed)
	state.PVID = int64OrNull(state.PVID, iface.PVID)
	state.MTU = int64OrNull(state.MTU, iface.MTU)

	r.applyStatus(ctx, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *interfaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state interfaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.data == nil {
		resp.Diagnostics.Append(errNotConfigured("netgear_interface")...)
		return
	}

	port := plan.Port.ValueString()

	var settings []string
	if !plan.Description.Equal(state.Description) {
		if plan.Description.IsNull() {
			settings = append(settings, "no description")
		} else {
			settings = append(settings, "description "+quote(plan.Description.ValueString()))
		}
	}
	if !plan.Enabled.Equal(state.Enabled) {
		if plan.Enabled.ValueBool() {
			settings = append(settings, "no shutdown")
		} else {
			settings = append(settings, "shutdown")
		}
	}
	if !plan.Speed.Equal(state.Speed) {
		if plan.Speed.IsNull() {
			settings = append(settings, "no speed")
		} else {
			settings = append(settings, speedCommand(plan.Speed.ValueString()))
		}
	}
	if !plan.PVID.Equal(state.PVID) {
		// Clearing the pvid returns the port to the default VLAN.
		pvid := int64(defaultPVID)
		if !plan.PVID.IsNull() {
			pvid = plan.PVID.ValueInt64()
		}
		settings = append(settings, "vlan pvid "+itoa(pvid))
	}
	if !plan.MTU.Equal(state.MTU) {
		if plan.MTU.IsNull() {
			settings = append(settings, "no mtu")
		} else {
			settings = append(settings, "mtu "+itoa(plan.MTU.ValueInt64()))
		}
	}
	if !plan.FlowControl.Equal(state.FlowControl) {
		if plan.FlowControl.IsNull() {
			settings = append(settings, "no flowcontrol")
		} else {
			settings = append(settings, flowControlCommand(plan.FlowControl.ValueBool()))
		}
	}

	if len(settings) > 0 {
		cmds := append([]string{"configure", "interface " + port}, settings...)
		cmds = append(cmds, "exit", "exit")

		if _, err := r.data.apply(ctx, cmds...); err != nil {
			resp.Diagnostics.AddError("Unable to Update the Port", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(port)
	r.applyStatus(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *interfaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state interfaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.data == nil {
		resp.Diagnostics.Append(errNotConfigured("netgear_interface")...)
		return
	}

	// The port itself survives, so return what was managed to its default.
	cmds := []string{"configure", "interface " + state.Port.ValueString()}
	if !state.Description.IsNull() {
		cmds = append(cmds, "no description")
	}
	if !state.Enabled.ValueBool() {
		cmds = append(cmds, "no shutdown")
	}
	if !state.Speed.IsNull() {
		cmds = append(cmds, "no speed")
	}
	if !state.PVID.IsNull() {
		cmds = append(cmds, "vlan pvid "+itoa(defaultPVID))
	}
	if !state.MTU.IsNull() {
		cmds = append(cmds, "no mtu")
	}
	if !state.FlowControl.IsNull() {
		cmds = append(cmds, "no flowcontrol")
	}
	cmds = append(cmds, "exit", "exit")

	if _, err := r.data.apply(ctx, cmds...); err != nil {
		resp.Diagnostics.AddError("Unable to Reset the Port", err.Error())
	}
}

// applyStatus fills the computed attributes from `show interfaces status`. A
// switch that reports nothing for the port leaves them empty rather than unknown,
// which the framework rejects.
func (r *interfaceResource) applyStatus(ctx context.Context, model *interfaceResourceModel, diags *diag.Diagnostics) {
	status, err := r.data.readStatus(ctx, model.Port.ValueString())
	if err != nil {
		diags.AddError("Unable to Read the Port Status", err.Error())
		return
	}

	model.AdminStatus = types.StringValue(status.AdminStatus)
	model.LinkStatus = types.StringValue(status.LinkStatus)
}

// speedCommand translates a schema speed onto the FASTPATH form, which takes the
// duplex as a separate word.
func speedCommand(speed string) string {
	if speed == "auto" {
		return "auto negotiate"
	}

	rate, duplex, _ := strings.Cut(speed, "-")

	return "speed " + rate + " " + duplex
}

func flowControlCommand(enabled bool) string {
	if enabled {
		return "flowcontrol"
	}

	return "no flowcontrol"
}

func (r *interfaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("port"), req.ID)...)
}
