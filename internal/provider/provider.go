package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ provider.Provider = &netgearProvider{}

type netgearProvider struct {
	version string
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &netgearProvider{version: version}
	}
}

func (p *netgearProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "netgear"
	resp.Version = p.version
}

func (p *netgearProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Interact with NETGEAR devices.",
	}
}

func (p *netgearProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
}

func (p *netgearProvider) Resources(ctx context.Context) []func() resource.Resource {
	return nil
}

func (p *netgearProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return nil
}
