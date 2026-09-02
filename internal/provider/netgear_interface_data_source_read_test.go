package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// portLookup is the configuration a netgear_interface data source reads, with the
// element types a zero value set lacks filled in.
func portLookup(ctx context.Context, d datasource.DataSource, port string) tfsdk.Config {
	return configFor(ctx, d, interfaceDataSourceModel{
		Port:        types.StringValue(port),
		Vlans:       types.SetNull(types.Int64Type),
		TaggedVlans: types.SetNull(types.Int64Type),
	})
}

// ifaceFixture is a running config with one configured port and one in a LAG.
const ifaceFixture = `interface 0/1
description "workstation"
vlan pvid 10
mtu 9216
vlan participation include 10
vlan participation include 20
vlan tagging 20
exit
interface 0/2
shutdown
exit
interface 0/23
addport 3/1
exit
`

// showPortAll is `show port all` as the switch prints it, cut to the ports the
// specs care about.
const showPortAll = `                 Admin     Physical   Physical   Link   Link    LACP   Flow
Intf      Type   Mode      Mode       Status     Status Trap    Mode   Mode
--------- ------ --------- ---------- ---------- ------ ------- ------ -------
g1               Enable    Auto       1000 Full  Up     Enable  Enable Disable
g2               Disable   Auto                  Down   Enable  Enable Disable
g10              Enable    Auto                  Down   Enable  Enable Disable
g23              Enable    Auto       1000 Full  Up     Enable  Enable Disable
lag 1            Enable                          Down   Enable  N/A    Disable
`

var _ = Describe("InterfaceDataSource Read", func() {
	var (
		client *fakeClient
		d      *interfaceDataSource
	)

	BeforeEach(func() {
		client = &fakeClient{
			config: ifaceFixture,
			output: map[string]string{
				"show port 0/1": "g1  Enable  Auto  1000 Full  Up  Enable  Enable  Disable",
			},
		}
		d = &interfaceDataSource{data: testSwitch(client)}
	})

	It("should report the port configuration and its live status", func(ctx SpecContext) {
		resp := &datasource.ReadResponse{State: blankDataState(ctx, d)}

		d.Read(ctx, datasource.ReadRequest{Config: portLookup(ctx, d, "0/1")}, resp)

		Expect(resp.Diagnostics.HasError()).To(BeFalse(), "%v", resp.Diagnostics)

		var state interfaceDataSourceModel
		Expect(resp.State.Get(ctx, &state).HasError()).To(BeFalse())
		Expect(state.ID).To(Equal(types.StringValue("0/1")))
		Expect(state.Description).To(Equal(types.StringValue("workstation")))
		Expect(state.Enabled).To(Equal(types.BoolValue(true)))
		Expect(state.PVID).To(Equal(types.Int64Value(10)))
		Expect(state.MTU).To(Equal(types.Int64Value(9216)))
		Expect(state.Vlans).To(Equal(int64Set([]int64{10, 20})))
		Expect(state.TaggedVlans).To(Equal(int64Set([]int64{20})))
		Expect(state.AdminStatus).To(Equal(types.StringValue("enable")))
		Expect(state.LinkStatus).To(Equal(types.StringValue("up")))
		Expect(client.sent()).To(HaveExactElements("show port 0/1"))
	})

	It("should report the lag a port belongs to", func(ctx SpecContext) {
		client.output["show port 0/23"] = "g23  Enable  Auto  1000 Full  Up  Enable  Enable  Disable"
		resp := &datasource.ReadResponse{State: blankDataState(ctx, d)}

		d.Read(ctx, datasource.ReadRequest{Config: portLookup(ctx, d, "0/23")}, resp)

		var state interfaceDataSourceModel
		Expect(resp.State.Get(ctx, &state).HasError()).To(BeFalse())
		Expect(state.Lag).To(Equal(types.StringValue("3/1")))
	})

	It("should report a port the running config never mentions", func(ctx SpecContext) {
		client.output["show port 0/10"] = "g10  Enable  Auto  Down  Enable  Enable  Disable"
		resp := &datasource.ReadResponse{State: blankDataState(ctx, d)}

		d.Read(ctx, datasource.ReadRequest{Config: portLookup(ctx, d, "0/10")}, resp)

		Expect(resp.Diagnostics.HasError()).To(BeFalse(), "%v", resp.Diagnostics)

		var state interfaceDataSourceModel
		Expect(resp.State.Get(ctx, &state).HasError()).To(BeFalse())
		Expect(state.Port).To(Equal(types.StringValue("0/10")))
		Expect(state.Description).To(Equal(types.StringValue("")))
		Expect(state.Enabled).To(Equal(types.BoolValue(true)))
		Expect(state.AdminStatus).To(Equal(types.StringValue("enable")))
	})

	It("should error on a port the switch says nothing about", func(ctx SpecContext) {
		resp := &datasource.ReadResponse{State: blankDataState(ctx, d)}

		d.Read(ctx, datasource.ReadRequest{Config: portLookup(ctx, d, "0/99")}, resp)

		Expect(resp.Diagnostics.HasError()).To(BeTrue())
		Expect(resp.Diagnostics.Errors()[0].Summary()).To(Equal("Port Not Found"))
	})
})

var _ = Describe("InterfacesDataSource Read", func() {
	var (
		client *fakeClient
		d      *interfacesDataSource
	)

	BeforeEach(func() {
		client = &fakeClient{
			config: ifaceFixture,
			output: map[string]string{"show port all": showPortAll},
		}
		d = &interfacesDataSource{data: testSwitch(client)}
	})

	It("should list the physical ports in switch order", func(ctx SpecContext) {
		resp := &datasource.ReadResponse{State: blankDataState(ctx, d)}

		d.Read(ctx, datasource.ReadRequest{}, resp)

		Expect(resp.Diagnostics.HasError()).To(BeFalse(), "%v", resp.Diagnostics)

		var state interfacesDataSourceModel
		Expect(resp.State.Get(ctx, &state).HasError()).To(BeFalse())
		Expect(state.ID).To(Equal(types.StringValue("interfaces")))
		Expect(ports(state.Interfaces)).To(HaveExactElements("0/1", "0/2", "0/10", "0/23"))
		Expect(client.sent()).To(HaveExactElements("show port all"))
	})

	It("should join the running config onto the status", func(ctx SpecContext) {
		resp := &datasource.ReadResponse{State: blankDataState(ctx, d)}

		d.Read(ctx, datasource.ReadRequest{}, resp)

		var state interfacesDataSourceModel
		Expect(resp.State.Get(ctx, &state).HasError()).To(BeFalse())
		Expect(state.Interfaces[0].Description).To(Equal(types.StringValue("workstation")))
		Expect(state.Interfaces[0].LinkStatus).To(Equal(types.StringValue("up")))
		Expect(state.Interfaces[1].Enabled).To(Equal(types.BoolValue(false)))
		Expect(state.Interfaces[2].Description).To(Equal(types.StringValue("")))
	})
})

// ports reads the port ids off a list of interface entries.
func ports(entries []interfaceEntryModel) []string {
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.Port.ValueString())
	}

	return ids
}
