package provider_test

import (
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/unmango/terraform-provider-netgear/internal/provider"
)

var _ = Describe("InterfaceResource", func() {
	var schema rschema.Schema

	BeforeEach(func(ctx SpecContext) {
		schema = resourceSchema(ctx, provider.NewInterfaceResource())
	})

	It("should be named netgear_interface", func(ctx SpecContext) {
		r := provider.NewInterfaceResource()
		resp := &resource.MetadataResponse{}

		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "netgear"}, resp)

		Expect(resp.TypeName).To(Equal("netgear_interface"))
	})

	It("should require port", func() {
		Expect(schema.Attributes["port"].IsRequired()).To(BeTrue())
	})

	It("should compute link state", func() {
		Expect(schema.Attributes["admin_status"].IsComputed()).To(BeTrue())
		Expect(schema.Attributes["link_status"].IsComputed()).To(BeTrue())
	})

	It("should own pvid but not tagged membership", func() {
		Expect(schema.Attributes).To(HaveKey("pvid"))
		Expect(schema.Attributes).NotTo(HaveKey("tagged_vlans"))
	})
})
