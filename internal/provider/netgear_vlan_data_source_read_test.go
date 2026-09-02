package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// vlanLookup is the configuration a netgear_vlan data source reads, with the
// element types a zero value set lacks filled in.
func vlanLookup(ctx context.Context, d datasource.DataSource, id int64) tfsdk.Config {
	return configFor(ctx, d, vlanDataSourceModel{
		VlanID:        types.Int64Value(id),
		TaggedPorts:   types.SetNull(types.StringType),
		UntaggedPorts: types.SetNull(types.StringType),
	})
}

// vlanFixture is a running config with one named VLAN and two member ports.
const vlanFixture = `vlan database
vlan 10
vlan name 10 "mgmt"
vlan routing 10
exit
vlan database
vlan 20
exit
interface 0/1
vlan participation include 10
exit
interface 0/24
vlan participation include 10
vlan tagging 10
exit
`

var _ = Describe("VlanDataSource Read", func() {
	var (
		client *fakeClient
		d      *vlanDataSource
	)

	BeforeEach(func() {
		client = &fakeClient{config: vlanFixture}
		d = &vlanDataSource{data: testSwitch(client)}
	})

	It("should report the vlan and its membership", func(ctx SpecContext) {
		resp := &datasource.ReadResponse{State: blankDataState(ctx, d)}

		d.Read(ctx, datasource.ReadRequest{Config: vlanLookup(ctx, d, 10)}, resp)

		Expect(resp.Diagnostics.HasError()).To(BeFalse(), "%v", resp.Diagnostics)

		var state vlanDataSourceModel
		Expect(resp.State.Get(ctx, &state).HasError()).To(BeFalse())
		Expect(state.ID).To(Equal(types.StringValue("10")))
		Expect(state.Name).To(Equal(types.StringValue("mgmt")))
		Expect(state.Routing).To(Equal(types.BoolValue(true)))
		Expect(state.TaggedPorts).To(Equal(stringSet([]string{"0/24"})))
		Expect(state.UntaggedPorts).To(Equal(stringSet([]string{"0/1"})))
	})

	It("should report an empty vlan without inventing membership", func(ctx SpecContext) {
		resp := &datasource.ReadResponse{State: blankDataState(ctx, d)}

		d.Read(ctx, datasource.ReadRequest{Config: vlanLookup(ctx, d, 20)}, resp)

		var state vlanDataSourceModel
		Expect(resp.State.Get(ctx, &state).HasError()).To(BeFalse())
		Expect(state.Name).To(Equal(types.StringValue("")))
		Expect(state.TaggedPorts).To(Equal(stringSet(nil)))
	})

	It("should error on a vlan the switch does not have", func(ctx SpecContext) {
		resp := &datasource.ReadResponse{State: blankDataState(ctx, d)}

		d.Read(ctx, datasource.ReadRequest{Config: vlanLookup(ctx, d, 99)}, resp)

		Expect(resp.Diagnostics.HasError()).To(BeTrue())
		Expect(resp.Diagnostics.Errors()[0].Summary()).To(Equal("VLAN Not Found"))
	})

	It("should surface a switch that cannot be read", func(ctx SpecContext) {
		client.runErr = errSwitch
		resp := &datasource.ReadResponse{State: blankDataState(ctx, d)}

		d.Read(ctx, datasource.ReadRequest{Config: vlanLookup(ctx, d, 10)}, resp)

		Expect(resp.Diagnostics.HasError()).To(BeTrue())
		Expect(resp.Diagnostics.Errors()[0].Summary()).To(Equal("Unable to Read the Switch Configuration"))
	})
})

var _ = Describe("VlansDataSource Read", func() {
	var d *vlansDataSource

	BeforeEach(func() {
		d = &vlansDataSource{data: testSwitch(&fakeClient{config: vlanFixture})}
	})

	It("should list every vlan in ascending id order", func(ctx SpecContext) {
		resp := &datasource.ReadResponse{State: blankDataState(ctx, d)}

		d.Read(ctx, datasource.ReadRequest{}, resp)

		Expect(resp.Diagnostics.HasError()).To(BeFalse(), "%v", resp.Diagnostics)

		var state vlansDataSourceModel
		Expect(resp.State.Get(ctx, &state).HasError()).To(BeFalse())
		Expect(state.ID).To(Equal(types.StringValue("vlans")))
		Expect(state.Vlans).To(HaveLen(2))
		Expect(state.Vlans[0].VlanID).To(Equal(types.Int64Value(10)))
		Expect(state.Vlans[0].Name).To(Equal(types.StringValue("mgmt")))
		Expect(state.Vlans[1].VlanID).To(Equal(types.Int64Value(20)))
	})

	It("should report an empty list when the switch has no vlans", func(ctx SpecContext) {
		d = &vlansDataSource{data: testSwitch(&fakeClient{})}
		resp := &datasource.ReadResponse{State: blankDataState(ctx, d)}

		d.Read(ctx, datasource.ReadRequest{}, resp)

		var state vlansDataSourceModel
		Expect(resp.State.Get(ctx, &state).HasError()).To(BeFalse())
		Expect(state.Vlans).To(BeEmpty())
	})
})
