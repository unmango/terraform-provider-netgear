package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// withNullSets fills in the element type a zero value set lacks, so a spec can
// leave membership out of the model it builds.
func withNullSets(ctx context.Context, model vlanResourceModel) vlanResourceModel {
	if model.TaggedPorts.ElementType(ctx) == nil {
		model.TaggedPorts = types.SetNull(types.StringType)
	}
	if model.UntaggedPorts.ElementType(ctx) == nil {
		model.UntaggedPorts = types.SetNull(types.StringType)
	}

	return model
}

func vlanPlan(ctx context.Context, model vlanResourceModel) tfsdk.Plan {
	return planFor(ctx, &vlanResource{}, withNullSets(ctx, model))
}

func vlanState(ctx context.Context, model vlanResourceModel) tfsdk.State {
	return stateFor(ctx, &vlanResource{}, withNullSets(ctx, model))
}

func emptyState(ctx context.Context) tfsdk.State {
	return blankState(ctx, &vlanResource{})
}

func readVlanState(ctx context.Context, state tfsdk.State) vlanResourceModel {
	return readState[vlanResourceModel](ctx, state)
}

var _ = Describe("VlanResource CRUD", func() {
	var (
		client *fakeClient
		r      *vlanResource
	)

	BeforeEach(func() {
		client = &fakeClient{}
		r = &vlanResource{data: testSwitch(client)}
	})

	Describe("Create", func() {
		It("should build the vlan and its membership", func(ctx SpecContext) {
			plan := vlanPlan(ctx, vlanResourceModel{
				VlanID:        types.Int64Value(10),
				Name:          types.StringValue("mgmt"),
				Routing:       types.BoolValue(false),
				TaggedPorts:   stringSet([]string{"0/24"}),
				UntaggedPorts: stringSet([]string{"0/1"}),
			})
			resp := &resource.CreateResponse{State: emptyState(ctx)}

			r.Create(ctx, resource.CreateRequest{Plan: plan}, resp)

			Expect(resp.Diagnostics.HasError()).To(BeFalse(), "%v", resp.Diagnostics)
			Expect(client.sent()).To(HaveExactElements(
				"vlan database",
				"vlan 10",
				`vlan name 10 "mgmt"`,
				"exit",
				"configure",
				"interface 0/1",
				"vlan participation include 10",
				"exit",
				"interface 0/24",
				"vlan participation include 10",
				"vlan tagging 10",
				"exit",
				"exit",
			))
		})

		It("should set the id from the vlan id", func(ctx SpecContext) {
			plan := vlanPlan(ctx, vlanResourceModel{VlanID: types.Int64Value(20), Routing: types.BoolValue(false)})
			resp := &resource.CreateResponse{State: emptyState(ctx)}

			r.Create(ctx, resource.CreateRequest{Plan: plan}, resp)

			Expect(readVlanState(ctx, resp.State).ID).To(Equal(types.StringValue("20")))
		})

		It("should enable routing when asked", func(ctx SpecContext) {
			plan := vlanPlan(ctx, vlanResourceModel{VlanID: types.Int64Value(10), Routing: types.BoolValue(true)})
			resp := &resource.CreateResponse{State: emptyState(ctx)}

			r.Create(ctx, resource.CreateRequest{Plan: plan}, resp)

			Expect(client.sent()).To(ContainElement("vlan routing 10"))
		})

		It("should save the configuration", func(ctx SpecContext) {
			plan := vlanPlan(ctx, vlanResourceModel{VlanID: types.Int64Value(10), Routing: types.BoolValue(false)})
			resp := &resource.CreateResponse{State: emptyState(ctx)}

			r.Create(ctx, resource.CreateRequest{Plan: plan}, resp)

			Expect(client.saveCount()).To(Equal(1))
		})

		It("should leave NVRAM alone when saving is disabled", func(ctx SpecContext) {
			r.data.saveConfig = false
			plan := vlanPlan(ctx, vlanResourceModel{VlanID: types.Int64Value(10), Routing: types.BoolValue(false)})
			resp := &resource.CreateResponse{State: emptyState(ctx)}

			r.Create(ctx, resource.CreateRequest{Plan: plan}, resp)

			Expect(client.saveCount()).To(BeZero())
		})

		It("should report a command the switch rejected", func(ctx SpecContext) {
			client.runErr = errSwitch
			plan := vlanPlan(ctx, vlanResourceModel{VlanID: types.Int64Value(10), Routing: types.BoolValue(false)})
			resp := &resource.CreateResponse{State: emptyState(ctx)}

			r.Create(ctx, resource.CreateRequest{Plan: plan}, resp)

			Expect(resp.Diagnostics.HasError()).To(BeTrue())
			Expect(resp.Diagnostics.Errors()[0].Detail()).To(ContainSubstring("Invalid input detected"))
		})
	})

	Describe("Read", func() {
		BeforeEach(func() {
			client.config = `vlan database
vlan 10
vlan name 10 "mgmt"
exit
interface 0/1
vlan participation include 10
exit
interface 0/24
vlan participation include 10
vlan tagging 10
exit
`
		})

		It("should refresh membership from the running config", func(ctx SpecContext) {
			state := vlanState(ctx, vlanResourceModel{
				ID:            types.StringValue("10"),
				VlanID:        types.Int64Value(10),
				Name:          types.StringValue("old"),
				Routing:       types.BoolValue(false),
				TaggedPorts:   stringSet([]string{"0/2"}),
				UntaggedPorts: stringSet([]string{"0/2"}),
			})
			resp := &resource.ReadResponse{State: state}

			r.Read(ctx, resource.ReadRequest{State: state}, resp)

			refreshed := readVlanState(ctx, resp.State)

			Expect(resp.Diagnostics.HasError()).To(BeFalse(), "%v", resp.Diagnostics)
			Expect(refreshed.Name).To(Equal(types.StringValue("mgmt")))
			Expect(refreshed.TaggedPorts).To(Equal(stringSet([]string{"0/24"})))
			Expect(refreshed.UntaggedPorts).To(Equal(stringSet([]string{"0/1"})))
		})

		It("should keep membership null when the config never set it", func(ctx SpecContext) {
			client.config = "vlan database\nvlan 30\nexit\n"
			state := vlanState(ctx, vlanResourceModel{
				ID:      types.StringValue("30"),
				VlanID:  types.Int64Value(30),
				Routing: types.BoolValue(false),
			})
			resp := &resource.ReadResponse{State: state}

			r.Read(ctx, resource.ReadRequest{State: state}, resp)

			refreshed := readVlanState(ctx, resp.State)

			Expect(refreshed.TaggedPorts.IsNull()).To(BeTrue())
			Expect(refreshed.Name.IsNull()).To(BeTrue())
		})

		It("should drop a vlan the switch no longer has", func(ctx SpecContext) {
			state := vlanState(ctx, vlanResourceModel{
				ID:      types.StringValue("99"),
				VlanID:  types.Int64Value(99),
				Routing: types.BoolValue(false),
			})
			resp := &resource.ReadResponse{State: state}

			r.Read(ctx, resource.ReadRequest{State: state}, resp)

			Expect(resp.Diagnostics.HasError()).To(BeFalse())
			Expect(resp.State.Raw.IsNull()).To(BeTrue())
		})
	})

	Describe("Update", func() {
		It("should move a port from untagged to tagged without rejoining it", func(ctx SpecContext) {
			state := vlanState(ctx, vlanResourceModel{
				ID:            types.StringValue("10"),
				VlanID:        types.Int64Value(10),
				Routing:       types.BoolValue(false),
				UntaggedPorts: stringSet([]string{"0/1"}),
			})
			plan := vlanPlan(ctx, vlanResourceModel{
				ID:          types.StringValue("10"),
				VlanID:      types.Int64Value(10),
				Routing:     types.BoolValue(false),
				TaggedPorts: stringSet([]string{"0/1"}),
			})
			resp := &resource.UpdateResponse{State: state}

			r.Update(ctx, resource.UpdateRequest{Plan: plan, State: state}, resp)

			Expect(resp.Diagnostics.HasError()).To(BeFalse(), "%v", resp.Diagnostics)
			Expect(client.sent()).To(HaveExactElements(
				"configure", "interface 0/1", "vlan tagging 10", "exit", "exit",
			))
		})

		It("should exclude a port that left the vlan", func(ctx SpecContext) {
			state := vlanState(ctx, vlanResourceModel{
				ID:            types.StringValue("10"),
				VlanID:        types.Int64Value(10),
				Routing:       types.BoolValue(false),
				UntaggedPorts: stringSet([]string{"0/1", "0/2"}),
			})
			plan := vlanPlan(ctx, vlanResourceModel{
				ID:            types.StringValue("10"),
				VlanID:        types.Int64Value(10),
				Routing:       types.BoolValue(false),
				UntaggedPorts: stringSet([]string{"0/1"}),
			})
			resp := &resource.UpdateResponse{State: state}

			r.Update(ctx, resource.UpdateRequest{Plan: plan, State: state}, resp)

			Expect(client.sent()).To(ContainElement("vlan participation exclude 10"))
			Expect(client.sent()).NotTo(ContainElement("vlan participation include 10"))
		})

		It("should rename the vlan", func(ctx SpecContext) {
			state := vlanState(ctx, vlanResourceModel{
				ID: types.StringValue("10"), VlanID: types.Int64Value(10),
				Name: types.StringValue("old"), Routing: types.BoolValue(false),
			})
			plan := vlanPlan(ctx, vlanResourceModel{
				ID: types.StringValue("10"), VlanID: types.Int64Value(10),
				Name: types.StringValue("new"), Routing: types.BoolValue(false),
			})
			resp := &resource.UpdateResponse{State: state}

			r.Update(ctx, resource.UpdateRequest{Plan: plan, State: state}, resp)

			Expect(client.sent()).To(ContainElement(`vlan name 10 "new"`))
		})

		It("should send nothing when nothing changed", func(ctx SpecContext) {
			model := vlanResourceModel{
				ID: types.StringValue("10"), VlanID: types.Int64Value(10),
				Name: types.StringValue("mgmt"), Routing: types.BoolValue(false),
				UntaggedPorts: stringSet([]string{"0/1"}),
			}
			state := vlanState(ctx, model)
			resp := &resource.UpdateResponse{State: state}

			r.Update(ctx, resource.UpdateRequest{Plan: vlanPlan(ctx, model), State: state}, resp)

			Expect(client.sent()).To(BeEmpty())
			Expect(client.saveCount()).To(BeZero())
		})
	})

	Describe("Delete", func() {
		It("should remove the vlan from the database", func(ctx SpecContext) {
			state := vlanState(ctx, vlanResourceModel{
				ID: types.StringValue("10"), VlanID: types.Int64Value(10), Routing: types.BoolValue(false),
			})
			resp := &resource.DeleteResponse{State: state}

			r.Delete(ctx, resource.DeleteRequest{State: state}, resp)

			Expect(resp.Diagnostics.HasError()).To(BeFalse(), "%v", resp.Diagnostics)
			Expect(client.sent()).To(HaveExactElements(
				"vlan database", "no vlan 10", "exit",
			))
		})

		It("should refuse to delete vlan 1", func(ctx SpecContext) {
			state := vlanState(ctx, vlanResourceModel{
				ID: types.StringValue("1"), VlanID: types.Int64Value(1), Routing: types.BoolValue(false),
			})
			resp := &resource.DeleteResponse{State: state}

			r.Delete(ctx, resource.DeleteRequest{State: state}, resp)

			Expect(resp.Diagnostics.HasError()).To(BeTrue())
			Expect(client.sent()).To(BeEmpty())
		})
	})

	It("should report an unconfigured provider", func(ctx SpecContext) {
		r.data = nil
		plan := vlanPlan(ctx, vlanResourceModel{VlanID: types.Int64Value(10), Routing: types.BoolValue(false)})
		resp := &resource.CreateResponse{State: emptyState(ctx)}

		r.Create(ctx, resource.CreateRequest{Plan: plan}, resp)

		Expect(resp.Diagnostics.HasError()).To(BeTrue())
		Expect(resp.Diagnostics.Errors()[0].Summary()).To(Equal("Provider Not Configured"))
	})
})
