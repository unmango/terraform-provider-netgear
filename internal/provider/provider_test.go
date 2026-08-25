package provider_test

import (
	tfprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/UnstoppableMango/terraform-provider-netgear/internal/provider"
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
})
