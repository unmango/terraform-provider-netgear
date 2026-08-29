package provider_test

import (
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/UnstoppableMango/terraform-provider-netgear/internal/provider"
)

var _ = Describe("LagResource", func() {
	var schema rschema.Schema

	BeforeEach(func(ctx SpecContext) {
		schema = resourceSchema(ctx, provider.NewLagResource())
	})

	It("should be named netgear_lag", func(ctx SpecContext) {
		r := provider.NewLagResource()
		resp := &resource.MetadataResponse{}

		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "netgear"}, resp)

		Expect(resp.TypeName).To(Equal("netgear_lag"))
	})

	It("should require lag_id and members", func() {
		Expect(schema.Attributes["lag_id"].IsRequired()).To(BeTrue())
		Expect(schema.Attributes["members"].IsRequired()).To(BeTrue())
	})

	It("should expose the lag interface id for other resources", func() {
		Expect(schema.Attributes["interface_id"].IsComputed()).To(BeTrue())
	})
})
