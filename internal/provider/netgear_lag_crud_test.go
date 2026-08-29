package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func lagPlan(ctx context.Context, model lagResourceModel) tfsdk.Plan {
	return planFor(ctx, &lagResource{}, model)
}

func lagState(ctx context.Context, model lagResourceModel) tfsdk.State {
	return stateFor(ctx, &lagResource{}, model)
}

func readLagState(ctx context.Context, state tfsdk.State) lagResourceModel {
	return readState[lagResourceModel](ctx, state)
}

// lagOf is the minimum model the schema accepts: mode and enabled carry defaults
// and members is required.
func lagOf(id int64, members ...string) lagResourceModel {
	return lagResourceModel{
		LagID:   types.Int64Value(id),
		Mode:    types.StringValue(lagModeLACP),
		Enabled: types.BoolValue(true),
		Members: stringSet(members),
	}
}

var _ = Describe("LagResource CRUD", func() {
	var (
		client *fakeClient
		r      *lagResource
	)

	BeforeEach(func() {
		client = &fakeClient{}
		r = &lagResource{data: testSwitch(client)}
	})

	Describe("Create", func() {
		It("should build the group then add its members", func(ctx SpecContext) {
			model := lagOf(1, "0/23", "0/24")
			model.Name = types.StringValue("uplink")
			resp := &resource.CreateResponse{State: blankState(ctx, r)}

			r.Create(ctx, resource.CreateRequest{Plan: lagPlan(ctx, model)}, resp)

			Expect(resp.Diagnostics.HasError()).To(BeFalse(), "%v", resp.Diagnostics)
			Expect(client.sent()).To(HaveExactElements(
				"configure",
				"interface lag 1",
				`description "uplink"`,
				"no port-channel static",
				"no shutdown",
				"exit",
				"interface 0/23",
				"addport 3/1",
				"exit",
				"interface 0/24",
				"addport 3/1",
				"exit",
				"exit",
			))
		})

		It("should select static mode and a load balance selector", func(ctx SpecContext) {
			model := lagOf(2, "0/1")
			model.Mode = types.StringValue(lagModeStatic)
			model.HashMode = types.Int64Value(6)
			resp := &resource.CreateResponse{State: blankState(ctx, r)}

			r.Create(ctx, resource.CreateRequest{Plan: lagPlan(ctx, model)}, resp)

			Expect(client.sent()).To(ContainElements("port-channel static", "port-channel load-balance 6"))
		})

		It("should expose the interface id other resources reference", func(ctx SpecContext) {
			resp := &resource.CreateResponse{State: blankState(ctx, r)}

			r.Create(ctx, resource.CreateRequest{Plan: lagPlan(ctx, lagOf(3, "0/1"))}, resp)

			created := readLagState(ctx, resp.State)

			Expect(created.ID).To(Equal(types.StringValue("3")))
			Expect(created.InterfaceID).To(Equal(types.StringValue("3/3")))
		})
	})

	Describe("Read", func() {
		BeforeEach(func() {
			client.config = `interface lag 1
description "uplink"
exit
interface 0/23
addport 3/1
exit
interface 0/24
addport 3/1
exit
`
		})

		It("should refresh the group from the running config", func(ctx SpecContext) {
			state := lagState(ctx, lagOf(1, "0/23"))
			resp := &resource.ReadResponse{State: state}

			r.Read(ctx, resource.ReadRequest{State: state}, resp)

			refreshed := readLagState(ctx, resp.State)

			Expect(resp.Diagnostics.HasError()).To(BeFalse(), "%v", resp.Diagnostics)
			Expect(refreshed.Name).To(Equal(types.StringValue("uplink")))
			Expect(refreshed.Mode).To(Equal(types.StringValue(lagModeStatic)))
			Expect(refreshed.Members).To(Equal(stringSet([]string{"0/23", "0/24"})))
			Expect(refreshed.InterfaceID).To(Equal(types.StringValue("3/1")))
		})

		It("should drop a group the switch no longer has", func(ctx SpecContext) {
			state := lagState(ctx, lagOf(7, "0/1"))
			resp := &resource.ReadResponse{State: state}

			r.Read(ctx, resource.ReadRequest{State: state}, resp)

			Expect(resp.State.Raw.IsNull()).To(BeTrue())
		})
	})

	Describe("Update", func() {
		It("should move membership without touching the settled ports", func(ctx SpecContext) {
			current := lagOf(1, "0/23", "0/24")
			current.InterfaceID = types.StringValue("3/1")
			planned := lagOf(1, "0/23", "0/22")
			planned.InterfaceID = types.StringValue("3/1")

			resp := &resource.UpdateResponse{State: lagState(ctx, current)}

			r.Update(ctx, resource.UpdateRequest{
				Plan:  lagPlan(ctx, planned),
				State: lagState(ctx, current),
			}, resp)

			Expect(resp.Diagnostics.HasError()).To(BeFalse(), "%v", resp.Diagnostics)
			Expect(client.sent()).To(HaveExactElements(
				"configure",
				"interface 0/24",
				"deleteport 3/1",
				"exit",
				"interface 0/22",
				"addport 3/1",
				"exit",
				"exit",
			))
		})

		It("should switch the group to static", func(ctx SpecContext) {
			current := lagOf(1, "0/1")
			planned := lagOf(1, "0/1")
			planned.Mode = types.StringValue(lagModeStatic)

			resp := &resource.UpdateResponse{State: lagState(ctx, current)}

			r.Update(ctx, resource.UpdateRequest{
				Plan:  lagPlan(ctx, planned),
				State: lagState(ctx, current),
			}, resp)

			Expect(client.sent()).To(ContainElements("interface lag 1", "port-channel static"))
		})

		It("should send nothing when nothing changed", func(ctx SpecContext) {
			model := lagOf(1, "0/23")
			model.InterfaceID = types.StringValue("3/1")

			resp := &resource.UpdateResponse{State: lagState(ctx, model)}

			r.Update(ctx, resource.UpdateRequest{
				Plan:  lagPlan(ctx, model),
				State: lagState(ctx, model),
			}, resp)

			Expect(client.sent()).To(BeEmpty())
			Expect(client.saveCount()).To(BeZero())
		})
	})

	Describe("Delete", func() {
		It("should release the members and reset the group", func(ctx SpecContext) {
			model := lagOf(1, "0/23", "0/24")
			model.Name = types.StringValue("uplink")
			model.InterfaceID = types.StringValue("3/1")

			state := lagState(ctx, model)
			resp := &resource.DeleteResponse{State: state}

			r.Delete(ctx, resource.DeleteRequest{State: state}, resp)

			Expect(resp.Diagnostics.HasError()).To(BeFalse(), "%v", resp.Diagnostics)
			Expect(client.sent()).To(HaveExactElements(
				"configure",
				"interface 0/23",
				"deleteport 3/1",
				"exit",
				"interface 0/24",
				"deleteport 3/1",
				"exit",
				"interface lag 1",
				"no description",
				"port-channel static",
				"exit",
				"exit",
			))
		})

		It("should leave a group it never changed alone", func(ctx SpecContext) {
			model := lagOf(1, "0/23")
			model.Mode = types.StringValue(lagModeStatic)
			model.InterfaceID = types.StringValue("3/1")

			state := lagState(ctx, model)
			resp := &resource.DeleteResponse{State: state}

			r.Delete(ctx, resource.DeleteRequest{State: state}, resp)

			Expect(client.sent()).To(HaveExactElements(
				"configure", "interface 0/23", "deleteport 3/1", "exit", "exit",
			))
		})
	})

	It("should report an unconfigured provider", func(ctx SpecContext) {
		r.data = nil
		resp := &resource.CreateResponse{State: blankState(ctx, r)}

		r.Create(ctx, resource.CreateRequest{Plan: lagPlan(ctx, lagOf(1, "0/1"))}, resp)

		Expect(resp.Diagnostics.Errors()[0].Summary()).To(Equal("Provider Not Configured"))
	})
})
