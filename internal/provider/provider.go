package provider

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/UnstoppableMango/terraform-provider-netgear/internal/fastpath"
)

var _ provider.Provider = &netgearProvider{}

const (
	defaultPort     = 60000
	defaultUsername = "admin"
	defaultCLIFlow  = "auto"
	defaultTimeout  = 30 * time.Second
)

type netgearProvider struct {
	version string
}

type netgearProviderModel struct {
	Host           types.String `tfsdk:"host"`
	Port           types.Int64  `tfsdk:"port"`
	Username       types.String `tfsdk:"username"`
	Password       types.String `tfsdk:"password"`
	EnablePassword types.String `tfsdk:"enable_password"`
	CLIFlow        types.String `tfsdk:"cli_flow"`
	Timeout        types.String `tfsdk:"timeout"`
	SaveConfig     types.Bool   `tfsdk:"save_config"`
}

// clientConfig is the resolved connection configuration, after environment
// variable fallbacks and defaults have been applied.
type clientConfig struct {
	Host           string
	Port           int64
	Username       string
	Password       string
	EnablePassword string
	CLIFlow        string
	Timeout        time.Duration
	SaveConfig     bool
}

// newClient constructs the FASTPATH CLI client. It is a variable so tests can
// substitute a fake.
var newClient = func(ctx context.Context, cfg clientConfig) (Client, error) {
	return fastpath.New(fastpath.Config{
		Host:           cfg.Host,
		Port:           cfg.Port,
		Username:       cfg.Username,
		Password:       cfg.Password,
		EnablePassword: cfg.EnablePassword,
		Flow:           fastpath.Flow(cfg.CLIFlow),
		Timeout:        cfg.Timeout,
	})
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
		MarkdownDescription: "Interact with NETGEAR devices.\n\n" +
			"The provider drives the undocumented Broadcom FASTPATH CLI, which smart switches " +
			"expose over telnet on port 60000. The CLI is disabled by default: enable it in the " +
			"web UI under `Maintenance > Troubleshooting > Remote Diagnostics`.\n\n" +
			"~> Telnet provides no transport security and the `enable` step is effectively " +
			"unauthenticated. Reach the switch over a management VLAN only.",
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				MarkdownDescription: "Hostname or address of the switch. May also be set with the `NETGEAR_HOST` environment variable.",
				Optional:            true,
			},
			"port": schema.Int64Attribute{
				MarkdownDescription: "TCP port the FASTPATH CLI listens on. Defaults to `60000`. May also be set with the `NETGEAR_PORT` environment variable.",
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.Between(1, 65535),
				},
			},
			"username": schema.StringAttribute{
				MarkdownDescription: "Login user. Defaults to `admin`. May also be set with the `NETGEAR_USERNAME` environment variable.",
				Optional:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Login password, the same one used by the web UI. May also be set with the `NETGEAR_PASSWORD` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"enable_password": schema.StringAttribute{
				MarkdownDescription: "Password supplied at the `enable` prompt. Older firmware prompts and expects an empty string; newer firmware does not prompt at all. May also be set with the `NETGEAR_ENABLE_PASSWORD` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"cli_flow": schema.StringAttribute{
				MarkdownDescription: "Login flow to use: `auto` (default) detects it from the prompt, `modern` expects a `User:` prompt and no enable password, `legacy` expects the username prompt embedded in the interface configuration banner.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("auto", "modern", "legacy"),
				},
			},
			"timeout": schema.StringAttribute{
				MarkdownDescription: "Timeout for connecting and for each command, as a Go duration. Defaults to `30s`.",
				Optional:            true,
			},
			"save_config": schema.BoolAttribute{
				MarkdownDescription: "Write the running configuration to NVRAM after each change. Defaults to `true`. FASTPATH applies changes live, so without this they are lost on reboot.",
				Optional:            true,
			},
		},
	}
}

func (p *netgearProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config netgearProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	for name, value := range map[string]types.String{
		"host":            config.Host,
		"username":        config.Username,
		"password":        config.Password,
		"enable_password": config.EnablePassword,
		"cli_flow":        config.CLIFlow,
		"timeout":         config.Timeout,
	} {
		if value.IsUnknown() {
			resp.Diagnostics.AddAttributeError(
				path.Root(name),
				"Unknown Provider Configuration Value",
				"The provider cannot connect to the switch because "+name+" is not known at plan time. "+
					"Set it to a static value or apply the resource that produces it first.",
			)
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}

	resolved := clientConfig{
		Host:           stringOrEnv(config.Host, "NETGEAR_HOST", ""),
		Port:           defaultPort,
		Username:       stringOrEnv(config.Username, "NETGEAR_USERNAME", defaultUsername),
		Password:       stringOrEnv(config.Password, "NETGEAR_PASSWORD", ""),
		EnablePassword: stringOrEnv(config.EnablePassword, "NETGEAR_ENABLE_PASSWORD", ""),
		CLIFlow:        stringOrEnv(config.CLIFlow, "NETGEAR_CLI_FLOW", defaultCLIFlow),
		Timeout:        defaultTimeout,
		SaveConfig:     config.SaveConfig.IsNull() || config.SaveConfig.ValueBool(),
	}

	if !config.Port.IsNull() {
		resolved.Port = config.Port.ValueInt64()
	} else if env := os.Getenv("NETGEAR_PORT"); env != "" {
		port, err := strconv.ParseInt(env, 10, 64)
		if err != nil {
			resp.Diagnostics.AddError(
				"Invalid NETGEAR_PORT",
				"NETGEAR_PORT must be a number, got "+env+".",
			)
			return
		}
		resolved.Port = port
	}

	if !config.Timeout.IsNull() {
		timeout, err := time.ParseDuration(config.Timeout.ValueString())
		if err != nil {
			resp.Diagnostics.AddAttributeError(
				path.Root("timeout"),
				"Invalid Timeout",
				"timeout must be a Go duration such as \"30s\": "+err.Error(),
			)
			return
		}
		resolved.Timeout = timeout
	}

	if resolved.Host == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Missing Switch Host",
			"Set the host attribute or the NETGEAR_HOST environment variable.",
		)
	}
	if resolved.Password == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("password"),
			"Missing Switch Password",
			"Set the password attribute or the NETGEAR_PASSWORD environment variable.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := newClient(ctx, resolved)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Configure the Switch Client", err.Error())
		return
	}

	// Log in once up front so an unreachable switch, a disabled CLI, or wrong
	// credentials are reported here rather than from the first resource.
	if p, ok := client.(prober); ok {
		if err := p.Probe(ctx); err != nil {
			resp.Diagnostics.AddError(
				"Unable to Reach the Switch CLI",
				"Connecting to "+resolved.Host+" on port "+strconv.FormatInt(resolved.Port, 10)+" failed: "+err.Error()+"\n\n"+
					"If the connection was refused while the switch still answers ICMP, the CLI is disabled. "+
					"Enable it in the web UI under Maintenance > Troubleshooting > Remote Diagnostics.",
			)
			return
		}
	}

	data := &switchData{client: client, saveConfig: resolved.SaveConfig}
	resp.ResourceData = data
	resp.DataSourceData = data
}

func (p *netgearProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewVlanResource,
		NewInterfaceResource,
		NewLagResource,
	}
}

func (p *netgearProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewVlanDataSource,
		NewVlansDataSource,
		NewInterfaceDataSource,
		NewInterfacesDataSource,
		NewLagDataSource,
		NewLagsDataSource,
	}
}

func stringOrEnv(value types.String, env, fallback string) string {
	if !value.IsNull() {
		return value.ValueString()
	}
	if v := os.Getenv(env); v != "" {
		return v
	}
	return fallback
}
