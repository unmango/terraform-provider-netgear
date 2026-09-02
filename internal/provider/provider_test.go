package provider_test

import (
	tfprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/unmango/terraform-provider-netgear/internal/provider"
)

var testProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"netgear": providerserver.NewProtocol6WithError(provider.New("test")()),
}

var _ = Describe("Provider", func() {
	It("should report the netgear type name", func(ctx SpecContext) {
		p := provider.New("test")()
		resp := &tfprovider.MetadataResponse{}

		p.Metadata(ctx, tfprovider.MetadataRequest{}, resp)

		Expect(resp.TypeName).To(Equal("netgear"))
		Expect(resp.Version).To(Equal("test"))
	})

	It("should serve the protocol 6 provider", func() {
		server, err := testProviderFactories["netgear"]()

		Expect(err).NotTo(HaveOccurred())
		Expect(server).NotTo(BeNil())
	})

	It("should describe the CLI connection", func(ctx SpecContext) {
		p := provider.New("test")()
		resp := &tfprovider.SchemaResponse{}

		p.Schema(ctx, tfprovider.SchemaRequest{}, resp)

		Expect(resp.Diagnostics.HasError()).To(BeFalse())
		Expect(resp.Schema.Attributes).To(HaveKey("host"))
		Expect(resp.Schema.Attributes).To(HaveKey("port"))
		Expect(resp.Schema.Attributes).To(HaveKey("cli_flow"))
		Expect(resp.Schema.Attributes).To(HaveKey("save_config"))
	})

	It("should mark credentials sensitive", func(ctx SpecContext) {
		p := provider.New("test")()
		resp := &tfprovider.SchemaResponse{}

		p.Schema(ctx, tfprovider.SchemaRequest{}, resp)

		Expect(resp.Schema.Attributes["password"].IsSensitive()).To(BeTrue())
		Expect(resp.Schema.Attributes["enable_password"].IsSensitive()).To(BeTrue())
	})

	It("should register the switch resources", func(ctx SpecContext) {
		p := provider.New("test")()

		resources := p.Resources(ctx)

		Expect(resources).To(HaveLen(3))
	})

	It("should register the switch data sources", func(ctx SpecContext) {
		p := provider.New("test")()

		dataSources := p.DataSources(ctx)

		Expect(dataSources).To(HaveLen(6))
	})
})
