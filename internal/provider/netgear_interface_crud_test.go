package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func ifacePlan(ctx context.Context, model interfaceResourceModel) tfsdk.Plan {
	return planFor(ctx, &interfaceResource{}, model)
}

func ifaceState(ctx context.Context, model interfaceResourceModel) tfsdk.State {
	return stateFor(ctx, &interfaceResource{}, model)
}

func readIfaceState(ctx context.Context, state tfsdk.State) interfaceResourceModel {
	return readState[interfaceResourceModel](ctx, state)
}

// enabledPort is the minimum model the framework requires: enabled has a default
// and the computed attributes must not be unknown by the time state is written.
func enabledPort(port string) interfaceResourceModel {
	return interfaceResourceModel{
		Port:    types.StringValue(port),
		Enabled: types.BoolValue(true),
	}
}

var _ = Describe("InterfaceResource CRUD", func() {
	var (
		client *fakeClient
		r      *interfaceResource
	)

	BeforeEach(func() {
		client = &fakeClient{}
		r = &interfaceResource{data: testSwitch(client)}
	})

	Describe("Create", func() {
		It("should apply every configured setting", func(ctx SpecContext) {
			model := enabledPort("0/1")
			model.Description = types.StringValue("workstation")
			model.Speed = types.StringValue("1000-full")
			model.PVID = types.Int64Value(10)
			model.MTU = types.Int64Value(9216)
			resp := &resource.CreateResponse{State: blankState(ctx, r)}

			r.Create(ctx, resource.CreateRequest{Plan: ifacePlan(ctx, model)}, resp)

			Expect(resp.Diagnostics.HasError()).To(BeFalse(), "%v", resp.Diagnostics)
			Expect(client.sent()).To(HaveExactElements(
				"configure",
				"interface 0/1",
				`description "workstation"`,
				"no shutdown",
				"speed 1000 full",
				"vlan pvid 10",
				"mtu 9216",
				"exit",
				"exit",
				"show port 0/1",
			))
		})

		It("should shut down a disabled port", func(ctx SpecContext) {
			model := enabledPort("0/2")
			model.Enabled = types.BoolValue(false)
			resp := &resource.CreateResponse{State: blankState(ctx, r)}

			r.Create(ctx, resource.CreateRequest{Plan: ifacePlan(ctx, model)}, resp)

			Expect(client.sent()).To(ContainElement("shutdown"))
			Expect(client.sent()).NotTo(ContainElement("no shutdown"))
		})

		It("should translate auto speed", func(ctx SpecContext) {
			model := enabledPort("0/3")
			model.Speed = types.StringValue("auto")
			resp := &resource.CreateResponse{State: blankState(ctx, r)}

			r.Create(ctx, resource.CreateRequest{Plan: ifacePlan(ctx, model)}, resp)

			Expect(client.sent()).To(ContainElement("auto negotiate"))
		})

		It("should fill the computed status from the switch", func(ctx SpecContext) {
			client.output = map[string]string{
				"show port 0/1": "g1  Enable  Auto  1000 Full  Up  Enable  Enable  Disable",
			}
			resp := &resource.CreateResponse{State: blankState(ctx, r)}

			r.Create(ctx, resource.CreateRequest{Plan: ifacePlan(ctx, enabledPort("0/1"))}, resp)

			created := readIfaceState(ctx, resp.State)

			Expect(created.ID).To(Equal(types.StringValue("0/1")))
			Expect(created.AdminStatus).To(Equal(types.StringValue("enable")))
			Expect(created.LinkStatus).To(Equal(types.StringValue("up")))
		})

		It("should leave the status empty when the switch says nothing", func(ctx SpecContext) {
			resp := &resource.CreateResponse{State: blankState(ctx, r)}

			r.Create(ctx, resource.CreateRequest{Plan: ifacePlan(ctx, enabledPort("0/1"))}, resp)

			created := readIfaceState(ctx, resp.State)

			Expect(created.LinkStatus).To(Equal(types.StringValue("")))
		})
	})

	Describe("Read", func() {
		BeforeEach(func() {
			client.config = `interface 0/1
description "workstation"
vlan pvid 10
mtu 9216
exit
interface 0/2
shutdown
exit
`
			client.output = map[string]string{
				"show port 0/1": "g1  Enable  Auto  Down  Enable  Enable  Disable",
			}
		})

		It("should refresh from the running config", func(ctx SpecContext) {
			state := ifaceState(ctx, enabledPort("0/1"))
			resp := &resource.ReadResponse{State: state}

			r.Read(ctx, resource.ReadRequest{State: state}, resp)

			refreshed := readIfaceState(ctx, resp.State)

			Expect(resp.Diagnostics.HasError()).To(BeFalse(), "%v", resp.Diagnostics)
			Expect(refreshed.Description).To(Equal(types.StringValue("workstation")))
			Expect(refreshed.PVID).To(Equal(types.Int64Value(10)))
			Expect(refreshed.MTU).To(Equal(types.Int64Value(9216)))
			Expect(refreshed.LinkStatus).To(Equal(types.StringValue("down")))
		})

		It("should report a shut down port as disabled", func(ctx SpecContext) {
			state := ifaceState(ctx, enabledPort("0/2"))
			resp := &resource.ReadResponse{State: state}

			r.Read(ctx, resource.ReadRequest{State: state}, resp)

			Expect(readIfaceState(ctx, resp.State).Enabled).To(Equal(types.BoolValue(false)))
		})

		It("should keep unset attributes null", func(ctx SpecContext) {
			state := ifaceState(ctx, enabledPort("0/2"))
			resp := &resource.ReadResponse{State: state}

			r.Read(ctx, resource.ReadRequest{State: state}, resp)

			refreshed := readIfaceState(ctx, resp.State)

			Expect(refreshed.Description.IsNull()).To(BeTrue())
			Expect(refreshed.MTU.IsNull()).To(BeTrue())
		})

		It("should drop a port the switch does not report", func(ctx SpecContext) {
			state := ifaceState(ctx, enabledPort("0/48"))
			resp := &resource.ReadResponse{State: state}

			r.Read(ctx, resource.ReadRequest{State: state}, resp)

			Expect(resp.State.Raw.IsNull()).To(BeTrue())
		})
	})

	Describe("Update", func() {
		It("should send only what changed", func(ctx SpecContext) {
			current := enabledPort("0/1")
			current.Description = types.StringValue("old")
			current.PVID = types.Int64Value(10)

			planned := current
			planned.Description = types.StringValue("new")

			resp := &resource.UpdateResponse{State: ifaceState(ctx, current)}

			r.Update(ctx, resource.UpdateRequest{
				Plan:  ifacePlan(ctx, planned),
				State: ifaceState(ctx, current),
			}, resp)

			Expect(resp.Diagnostics.HasError()).To(BeFalse(), "%v", resp.Diagnostics)
			Expect(client.sent()).To(HaveExactElements(
				"configure",
				"interface 0/1",
				`description "new"`,
				"exit",
				"exit",
				"show port 0/1",
			))
		})

		It("should clear an attribute the configuration dropped", func(ctx SpecContext) {
			current := enabledPort("0/1")
			current.Description = types.StringValue("old")
			current.MTU = types.Int64Value(9216)

			planned := enabledPort("0/1")

			resp := &resource.UpdateResponse{State: ifaceState(ctx, current)}

			r.Update(ctx, resource.UpdateRequest{
				Plan:  ifacePlan(ctx, planned),
				State: ifaceState(ctx, current),
			}, resp)

			Expect(client.sent()).To(ContainElements("no description", "no mtu"))
		})

		It("should return a cleared pvid to the default vlan", func(ctx SpecContext) {
			current := enabledPort("0/1")
			current.PVID = types.Int64Value(10)

			resp := &resource.UpdateResponse{State: ifaceState(ctx, current)}

			r.Update(ctx, resource.UpdateRequest{
				Plan:  ifacePlan(ctx, enabledPort("0/1")),
				State: ifaceState(ctx, current),
			}, resp)

			Expect(client.sent()).To(ContainElement("vlan pvid 1"))
		})

		It("should configure nothing when nothing changed", func(ctx SpecContext) {
			model := enabledPort("0/1")
			model.Description = types.StringValue("same")

			resp := &resource.UpdateResponse{State: ifaceState(ctx, model)}

			r.Update(ctx, resource.UpdateRequest{
				Plan:  ifacePlan(ctx, model),
				State: ifaceState(ctx, model),
			}, resp)

			Expect(client.sent()).To(HaveExactElements("show port 0/1"))
			Expect(client.saveCount()).To(BeZero())
		})
	})

	Describe("Delete", func() {
		It("should reset the settings it managed", func(ctx SpecContext) {
			model := enabledPort("0/1")
			model.Description = types.StringValue("workstation")
			model.Enabled = types.BoolValue(false)
			model.PVID = types.Int64Value(10)

			state := ifaceState(ctx, model)
			resp := &resource.DeleteResponse{State: state}

			r.Delete(ctx, resource.DeleteRequest{State: state}, resp)

			Expect(resp.Diagnostics.HasError()).To(BeFalse(), "%v", resp.Diagnostics)
			Expect(client.sent()).To(HaveExactElements(
				"configure",
				"interface 0/1",
				"no description",
				"no shutdown",
				"vlan pvid 1",
				"exit",
				"exit",
			))
		})

		It("should leave an untouched port alone", func(ctx SpecContext) {
			state := ifaceState(ctx, enabledPort("0/1"))
			resp := &resource.DeleteResponse{State: state}

			r.Delete(ctx, resource.DeleteRequest{State: state}, resp)

			Expect(client.sent()).To(HaveExactElements("configure", "interface 0/1", "exit", "exit"))
		})
	})

	It("should report an unconfigured provider", func(ctx SpecContext) {
		r.data = nil
		resp := &resource.CreateResponse{State: blankState(ctx, r)}

		r.Create(ctx, resource.CreateRequest{Plan: ifacePlan(ctx, enabledPort("0/1"))}, resp)

		Expect(resp.Diagnostics.Errors()[0].Summary()).To(Equal("Provider Not Configured"))
	})
})
