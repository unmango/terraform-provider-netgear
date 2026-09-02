package provider_test

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/UnstoppableMango/terraform-provider-netgear/internal/provider"
)

// typeNameOf reports the type name a data source registers under.
func typeNameOf(ctx SpecContext, d datasource.DataSource) string {
	GinkgoHelper()

	resp := &datasource.MetadataResponse{}
	d.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "netgear"}, resp)

	return resp.TypeName
}

// nestedAttributes reads the attributes of a list nested attribute.
func nestedAttributes(schema dschema.Schema, name string) map[string]dschema.Attribute {
	GinkgoHelper()

	list, ok := schema.Attributes[name].(dschema.ListNestedAttribute)
	Expect(ok).To(BeTrue(), "%s is not a list of nested objects", name)

	return list.NestedObject.Attributes
}

var _ = Describe("Data sources", func() {
	DescribeTable("type names",
		func(ctx SpecContext, d datasource.DataSource, expected string) {
			Expect(typeNameOf(ctx, d)).To(Equal(expected))
		},
		Entry("vlan", provider.NewVlanDataSource(), "netgear_vlan"),
		Entry("vlans", provider.NewVlansDataSource(), "netgear_vlans"),
		Entry("interface", provider.NewInterfaceDataSource(), "netgear_interface"),
		Entry("interfaces", provider.NewInterfacesDataSource(), "netgear_interfaces"),
		Entry("lag", provider.NewLagDataSource(), "netgear_lag"),
		Entry("lags", provider.NewLagsDataSource(), "netgear_lags"),
	)

	DescribeTable("lookup keys",
		func(ctx SpecContext, d datasource.DataSource, key string) {
			schema := dataSourceSchema(ctx, d)

			Expect(schema.Attributes[key].IsRequired()).To(BeTrue())
			Expect(schema.Attributes["id"].IsComputed()).To(BeTrue())
		},
		Entry("vlan takes a vlan id", provider.NewVlanDataSource(), "vlan_id"),
		Entry("interface takes a port", provider.NewInterfaceDataSource(), "port"),
		Entry("lag takes a lag id", provider.NewLagDataSource(), "lag_id"),
	)

	DescribeTable("lists",
		func(ctx SpecContext, d datasource.DataSource, name string, keys ...string) {
			schema := dataSourceSchema(ctx, d)

			Expect(schema.Attributes[name].IsComputed()).To(BeTrue())

			attributes := nestedAttributes(schema, name)
			for _, key := range keys {
				Expect(attributes).To(HaveKey(key))
				Expect(attributes[key].IsComputed()).To(BeTrue())
			}
		},
		Entry("vlans", provider.NewVlansDataSource(), "vlans", "vlan_id", "tagged_ports", "untagged_ports"),
		Entry("interfaces", provider.NewInterfacesDataSource(), "interfaces", "port", "admin_status", "link_status", "lag"),
		Entry("lags", provider.NewLagsDataSource(), "lags", "lag_id", "members", "interface_id"),
	)

	It("should report port status on the interface data source", func(ctx SpecContext) {
		schema := dataSourceSchema(ctx, provider.NewInterfaceDataSource())

		Expect(schema.Attributes["admin_status"].IsComputed()).To(BeTrue())
		Expect(schema.Attributes["link_status"].IsComputed()).To(BeTrue())
	})
})
