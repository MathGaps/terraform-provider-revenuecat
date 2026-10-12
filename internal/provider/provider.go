// Package provider implements the RevenueCat Terraform provider.
package provider

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/nmehlei/terraform-provider-revenuecat/internal/revenuecat"
)

// Environment variables the provider reads when the corresponding argument is
// absent from configuration.
const (
	envAPIKey  = "REVENUECAT_API_KEY"
	envBaseURL = "REVENUECAT_BASE_URL"
)

// Ensure the implementation satisfies the framework interface.
var _ provider.Provider = (*revenueCatProvider)(nil)

type revenueCatProvider struct {
	// version is set at build time and reported in the User-Agent.
	version string
}

// New returns a function that constructs the provider for the given version.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &revenueCatProvider{version: version}
	}
}

// providerModel mirrors the provider configuration block.
type providerModel struct {
	APIKey                types.String `tfsdk:"api_key"`
	BaseURL               types.String `tfsdk:"base_url"`
	MaxRetries            types.Int64  `tfsdk:"max_retries"`
	RequestTimeoutSeconds types.Int64  `tfsdk:"request_timeout_seconds"`
}

func (p *revenueCatProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "revenuecat"
	resp.Version = p.version
}

func (p *revenueCatProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage the RevenueCat project catalog — apps, products, entitlements, " +
			"offerings and packages — through the RevenueCat REST API v2.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				MarkdownDescription: "A RevenueCat API v2 secret key. May also be set with the `" +
					envAPIKey + "` environment variable. The key needs write permission for the " +
					"project objects being managed.",
				Optional:  true,
				Sensitive: true,
			},
			"base_url": schema.StringAttribute{
				MarkdownDescription: "Base URL of the RevenueCat API. May also be set with the `" +
					envBaseURL + "` environment variable. Defaults to `" + revenuecat.DefaultBaseURL + "`.",
				Optional: true,
			},
			"max_retries": schema.Int64Attribute{
				MarkdownDescription: "How many times a rate-limited or server-error response is " +
					"retried before failing. Defaults to `" + strconv.Itoa(revenuecat.DefaultMaxRetries) + "`.",
				Optional: true,
			},
			"request_timeout_seconds": schema.Int64Attribute{
				MarkdownDescription: "Timeout in seconds for a single API request. Defaults to `" +
					strconv.Itoa(int(revenuecat.DefaultTimeout/time.Second)) + "`.",
				Optional: true,
			},
		},
	}
}

func (p *revenueCatProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// A value that is still unknown at plan time may be supplied by another
	// resource during apply, so defer rather than reporting it as missing.
	if config.APIKey.IsUnknown() || config.BaseURL.IsUnknown() ||
		config.MaxRetries.IsUnknown() || config.RequestTimeoutSeconds.IsUnknown() {
		return
	}

	apiKey := strings.TrimSpace(config.APIKey.ValueString())
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv(envAPIKey))
	}
	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing RevenueCat API key",
			"The provider requires a RevenueCat API v2 secret key. Set the `api_key` argument in the "+
				"provider configuration, or set the `"+envAPIKey+"` environment variable.",
		)
		return
	}

	baseURL := strings.TrimSpace(config.BaseURL.ValueString())
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv(envBaseURL))
	}
	if baseURL == "" {
		baseURL = revenuecat.DefaultBaseURL
	}
	if _, err := revenuecat.ParseBaseURL(baseURL); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("base_url"),
			"Invalid RevenueCat base URL",
			"The `base_url` argument must be an absolute http or https URL.\n\n"+err.Error(),
		)
		return
	}

	maxRetries := revenuecat.DefaultMaxRetries
	if !config.MaxRetries.IsNull() {
		value := config.MaxRetries.ValueInt64()
		if value < 0 {
			resp.Diagnostics.AddAttributeError(
				path.Root("max_retries"),
				"Invalid retry count",
				"The `max_retries` argument must not be negative.",
			)
			return
		}
		maxRetries = int(value)
	}

	timeout := revenuecat.DefaultTimeout
	if !config.RequestTimeoutSeconds.IsNull() {
		value := config.RequestTimeoutSeconds.ValueInt64()
		if value < 0 {
			resp.Diagnostics.AddAttributeError(
				path.Root("request_timeout_seconds"),
				"Invalid request timeout",
				"The `request_timeout_seconds` argument must not be negative.",
			)
			return
		}
		if value > 0 {
			timeout = time.Duration(value) * time.Second
		}
	}

	version := p.version
	if version == "" {
		version = "dev"
	}

	client, err := revenuecat.New(
		apiKey,
		baseURL,
		revenuecat.WithMaxRetries(maxRetries),
		revenuecat.WithTimeout(timeout),
		revenuecat.WithUserAgent("terraform-provider-revenuecat/"+version),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create the RevenueCat API client",
			err.Error(),
		)
		return
	}

	tflog.Debug(ctx, "configured the RevenueCat client", map[string]any{
		"base_url":    client.BaseURL(),
		"max_retries": maxRetries,
	})

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *revenueCatProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewAppResource,
		NewProductResource,
		NewEntitlementResource,
		NewOfferingResource,
		NewPackageResource,
		NewEntitlementProductAttachmentResource,
		NewPackageProductAttachmentResource,
		NewWebhookIntegrationResource,
	}
}

func (p *revenueCatProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewProjectDataSource,
		NewProjectsDataSource,
		NewAppDataSource,
		NewProductDataSource,
		NewEntitlementDataSource,
		NewOfferingDataSource,
		NewPackageDataSource,
	}
}
