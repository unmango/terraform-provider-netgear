package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// lagLookup is the configuration a netgear_lag data source reads, with the
// element type a zero value set lacks filled in.
func lagLookup(ctx context.Context, d datasource.DataSource, id int64) tfsdk.Config {
	return configFor(ctx, d, lagDataSourceModel{
		LagID:   types.Int64Value(id),
		Members: types.SetNull(types.StringType),
	})
}

// lagFixture is a running config with two groups, one of them negotiating.
const lagFixture = `interface lag 1
description "uplink"
no port-channel static
port-channel load-balance 6
exit
interface lag 2
exit
interface 0/23
addport 3/1
exit
interface 0/24
addport 3/1
exit
`

var _ = Describe("LagDataSource Read", func() {
	var d *lagDataSource

	BeforeEach(func() {
		d = &lagDataSource{data: testSwitch(&fakeClient{config: lagFixture})}
	})

	It("should report the group and its members", func(ctx SpecContext) {
		resp := &datasource.ReadResponse{State: blankDataState(ctx, d)}

		d.Read(ctx, datasource.ReadRequest{Config: lagLookup(ctx, d, 1)}, resp)

		Expect(resp.Diagnostics.HasError()).To(BeFalse(), "%v", resp.Diagnostics)

		var state lagDataSourceModel
		Expect(resp.State.Get(ctx, &state).HasError()).To(BeFalse())
		Expect(state.ID).To(Equal(types.StringValue("1")))
		Expect(state.Name).To(Equal(types.StringValue("uplink")))
		Expect(state.Mode).To(Equal(types.StringValue(lagModeLACP)))
		Expect(state.HashMode).To(Equal(types.Int64Value(6)))
		Expect(state.Enabled).To(Equal(types.BoolValue(true)))
		Expect(state.Members).To(Equal(stringSet([]string{"0/23", "0/24"})))
		Expect(state.InterfaceID).To(Equal(types.StringValue("3/1")))
	})

	It("should error on a group the running config does not carry", func(ctx SpecContext) {
		resp := &datasource.ReadResponse{State: blankDataState(ctx, d)}

		d.Read(ctx, datasource.ReadRequest{Config: lagLookup(ctx, d, 7)}, resp)

		Expect(resp.Diagnostics.HasError()).To(BeTrue())
		Expect(resp.Diagnostics.Errors()[0].Summary()).To(Equal("LAG Not Found"))
	})
})

var _ = Describe("LagsDataSource Read", func() {
	var d *lagsDataSource

	BeforeEach(func() {
		d = &lagsDataSource{data: testSwitch(&fakeClient{config: lagFixture})}
	})

	It("should list the configured groups in ascending id order", func(ctx SpecContext) {
		resp := &datasource.ReadResponse{State: blankDataState(ctx, d)}

		d.Read(ctx, datasource.ReadRequest{}, resp)

		Expect(resp.Diagnostics.HasError()).To(BeFalse(), "%v", resp.Diagnostics)

		var state lagsDataSourceModel
		Expect(resp.State.Get(ctx, &state).HasError()).To(BeFalse())
		Expect(state.ID).To(Equal(types.StringValue("lags")))
		Expect(state.Lags).To(HaveLen(2))
		Expect(state.Lags[0].LagID).To(Equal(types.Int64Value(1)))
		Expect(state.Lags[1].LagID).To(Equal(types.Int64Value(2)))
		Expect(state.Lags[1].Mode).To(Equal(types.StringValue(lagModeStatic)))
	})

	It("should report an empty list when no group is configured", func(ctx SpecContext) {
		d = &lagsDataSource{data: testSwitch(&fakeClient{})}
		resp := &datasource.ReadResponse{State: blankDataState(ctx, d)}

		d.Read(ctx, datasource.ReadRequest{}, resp)

		var state lagsDataSourceModel
		Expect(resp.State.Get(ctx, &state).HasError()).To(BeFalse())
		Expect(state.Lags).To(BeEmpty())
	})
})
