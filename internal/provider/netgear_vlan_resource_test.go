package provider_test

import (
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/UnstoppableMango/terraform-provider-netgear/internal/provider"
)

var _ = Describe("VlanResource", func() {
	var schema rschema.Schema

	BeforeEach(func(ctx SpecContext) {
		schema = resourceSchema(ctx, provider.NewVlanResource())
	})

	It("should be named netgear_vlan", func(ctx SpecContext) {
		r := provider.NewVlanResource()
		resp := &resource.MetadataResponse{}

		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "netgear"}, resp)

		Expect(resp.TypeName).To(Equal("netgear_vlan"))
	})

	It("should require vlan_id", func() {
		Expect(schema.Attributes).To(HaveKey("vlan_id"))
		Expect(schema.Attributes["vlan_id"].IsRequired()).To(BeTrue())
	})

	It("should compute id", func() {
		Expect(schema.Attributes["id"].IsComputed()).To(BeTrue())
	})

	It("should carry membership on the vlan", func() {
		Expect(schema.Attributes).To(HaveKey("tagged_ports"))
		Expect(schema.Attributes).To(HaveKey("untagged_ports"))
	})

	It("should support import", func() {
		_, ok := provider.NewVlanResource().(resource.ResourceWithImportState)

		Expect(ok).To(BeTrue())
	})
})
