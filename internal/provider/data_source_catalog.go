package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nmehlei/terraform-provider-revenuecat/internal/revenuecat"
)

// findByKey resolves a natural key by listing candidates and matching on the
// key each one carries. It reports a miss with an error naming the key that was
// searched for, so a typo is obvious from the diagnostic alone.
func findByKey[T any](items []T, wanted string, keyOf func(T) string, objectType, keyName string) (*T, error) {
	for i := range items {
		if keyOf(items[i]) == wanted {
			return &items[i], nil
		}
	}
	return nil, fmt.Errorf("no %s with %s %q was found", objectType, keyName, wanted)
}

// --- App ------------------------------------------------------------------

var (
	_ datasource.DataSource              = (*appDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*appDataSource)(nil)
)

// NewAppDataSource returns the revenuecat_app data source.
func NewAppDataSource() datasource.DataSource {
	return &appDataSource{}
}

type appDataSource struct {
	client *revenuecat.Client
}

func (d *appDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app"
}

func (d *appDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up an app in a RevenueCat project by identifier or by name. " +
			"Exactly one of `id` or `name` must be set.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the project the app belongs to.",
				Required:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the app. Conflicts with `name`.",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the app. Conflicts with `id`.",
				Optional:            true,
				Computed:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Store the app belongs to.",
				Computed:            true,
			},
			"package_name": schema.StringAttribute{
				MarkdownDescription: "Play Store package identifier, when `type` is `play_store`.",
				Computed:            true,
			},
			"bundle_id": schema.StringAttribute{
				MarkdownDescription: "App Store bundle identifier, when `type` is `app_store`.",
				Computed:            true,
			},
			"created_at": schema.Int64Attribute{
				MarkdownDescription: "Creation time of the app, in milliseconds since the Unix epoch.",
				Computed:            true,
			},
		},
	}
}

func (d *appDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *appDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config appDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := config.ProjectID.ValueString()
	hasID := !config.ID.IsNull() && config.ID.ValueString() != ""
	hasName := !config.Name.IsNull() && config.Name.ValueString() != ""

	if err := requireExactlyOneSelector(hasID, hasName, "id", "name"); err != nil {
		resp.Diagnostics.AddError("Invalid app selector", err.Error())
		return
	}

	var app *revenuecat.App
	if hasID {
		found, err := d.client.GetApp(ctx, projectID, config.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Unable to read the RevenueCat app", err.Error())
			return
		}
		app = found
	} else {
		apps, err := d.client.ListApps(ctx, projectID)
		if err != nil {
			resp.Diagnostics.AddError("Unable to list RevenueCat apps", err.Error())
			return
		}
		found, err := findByKey(apps, config.Name.ValueString(), func(a revenuecat.App) string { return a.Name }, "app", "name")
		if err != nil {
			resp.Diagnostics.AddError("No matching RevenueCat app", err.Error())
			return
		}
		app = found
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, appToDataSourceModel(projectID, app))...)
}

// appDataSourceModel is deliberately narrower than appModel. The resource
// carries the write-only credential and its version, which are inputs a
// practitioner supplies; a data source has neither to read back.
type appDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	ProjectID   types.String `tfsdk:"project_id"`
	Name        types.String `tfsdk:"name"`
	Type        types.String `tfsdk:"type"`
	PackageName types.String `tfsdk:"package_name"`
	BundleID    types.String `tfsdk:"bundle_id"`
	CreatedAt   types.Int64  `tfsdk:"created_at"`
}

func appToDataSourceModel(projectID string, app *revenuecat.App) appDataSourceModel {
	if app.ProjectID != "" {
		projectID = app.ProjectID
	}
	packageName, bundleID := types.StringNull(), types.StringNull()
	if app.PlayStore != nil {
		packageName = types.StringValue(app.PlayStore.PackageName)
	}
	if app.AppStore != nil {
		bundleID = types.StringValue(app.AppStore.BundleID)
	}
	return appDataSourceModel{
		ID:          types.StringValue(app.ID),
		ProjectID:   types.StringValue(projectID),
		Name:        types.StringValue(app.Name),
		Type:        types.StringValue(app.Type),
		PackageName: packageName,
		BundleID:    bundleID,
		CreatedAt:   types.Int64Value(app.CreatedAt),
	}
}

// --- Product --------------------------------------------------------------

var (
	_ datasource.DataSource              = (*productDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*productDataSource)(nil)
)

// NewProductDataSource returns the revenuecat_product data source.
func NewProductDataSource() datasource.DataSource {
	return &productDataSource{}
}

type productDataSource struct {
	client *revenuecat.Client
}

func (d *productDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_product"
}

func (d *productDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a product in a RevenueCat project by identifier or by store " +
			"identifier. Exactly one of `id` or `store_identifier` must be set.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the project the product belongs to.",
				Required:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the product. Conflicts with `store_identifier`.",
				Optional:            true,
				Computed:            true,
			},
			"store_identifier": schema.StringAttribute{
				MarkdownDescription: "Product identifier in the underlying store. Conflicts with `id`.",
				Optional:            true,
				Computed:            true,
			},
			"app_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the app the product belongs to.",
				Computed:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Type of the product.",
				Computed:            true,
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name of the product.",
				Computed:            true,
			},
			"created_at": schema.Int64Attribute{
				MarkdownDescription: "Creation time of the product, in milliseconds since the Unix epoch.",
				Computed:            true,
			},
			"subscription_duration": schema.StringAttribute{
				MarkdownDescription: "Billing period of a subscription product in ISO-8601, when RevenueCat knows it.",
				Computed:            true,
			},
		},
	}
}

// productDataModel is the product data source's own model: it reads what the
// API returns, so it has no title, which RevenueCat never returns.
type productDataModel struct {
	ID                   types.String `tfsdk:"id"`
	ProjectID            types.String `tfsdk:"project_id"`
	AppID                types.String `tfsdk:"app_id"`
	StoreIdentifier      types.String `tfsdk:"store_identifier"`
	Type                 types.String `tfsdk:"type"`
	DisplayName          types.String `tfsdk:"display_name"`
	CreatedAt            types.Int64  `tfsdk:"created_at"`
	SubscriptionDuration types.String `tfsdk:"subscription_duration"`
}

func (d *productDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *productDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config productDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := config.ProjectID.ValueString()
	hasID := !config.ID.IsNull() && config.ID.ValueString() != ""
	hasKey := !config.StoreIdentifier.IsNull() && config.StoreIdentifier.ValueString() != ""

	if err := requireExactlyOneSelector(hasID, hasKey, "id", "store_identifier"); err != nil {
		resp.Diagnostics.AddError("Invalid product selector", err.Error())
		return
	}

	var product *revenuecat.Product
	if hasID {
		found, err := d.client.GetProduct(ctx, projectID, config.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Unable to read the RevenueCat product", err.Error())
			return
		}
		product = found
	} else {
		products, err := d.client.ListProducts(ctx, projectID)
		if err != nil {
			resp.Diagnostics.AddError("Unable to list RevenueCat products", err.Error())
			return
		}
		found, err := findByKey(products, config.StoreIdentifier.ValueString(),
			func(p revenuecat.Product) string { return p.StoreIdentifier }, "product", "store_identifier")
		if err != nil {
			resp.Diagnostics.AddError("No matching RevenueCat product", err.Error())
			return
		}
		product = found
	}

	model := productToModel(projectID, product, types.StringNull(), types.StringNull())
	resp.Diagnostics.Append(resp.State.Set(ctx, productDataModel{
		ID:                   model.ID,
		ProjectID:            model.ProjectID,
		AppID:                model.AppID,
		StoreIdentifier:      model.StoreIdentifier,
		Type:                 model.Type,
		DisplayName:          model.DisplayName,
		CreatedAt:            model.CreatedAt,
		SubscriptionDuration: model.SubscriptionDuration,
	})...)
}

// --- Entitlement ----------------------------------------------------------

var (
	_ datasource.DataSource              = (*entitlementDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*entitlementDataSource)(nil)
)

// NewEntitlementDataSource returns the revenuecat_entitlement data source.
func NewEntitlementDataSource() datasource.DataSource {
	return &entitlementDataSource{}
}

type entitlementDataSource struct {
	client *revenuecat.Client
}

func (d *entitlementDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_entitlement"
}

func (d *entitlementDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up an entitlement in a RevenueCat project by identifier or by " +
			"lookup key. Exactly one of `id` or `lookup_key` must be set.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the project the entitlement belongs to.",
				Required:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the entitlement. Conflicts with `lookup_key`.",
				Optional:            true,
				Computed:            true,
			},
			"lookup_key": schema.StringAttribute{
				MarkdownDescription: "Lookup key of the entitlement. Conflicts with `id`.",
				Optional:            true,
				Computed:            true,
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name of the entitlement.",
				Computed:            true,
			},
			"created_at": schema.Int64Attribute{
				MarkdownDescription: "Creation time of the entitlement, in milliseconds since the Unix epoch.",
				Computed:            true,
			},
		},
	}
}

func (d *entitlementDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *entitlementDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config entitlementModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := config.ProjectID.ValueString()
	hasID := !config.ID.IsNull() && config.ID.ValueString() != ""
	hasKey := !config.LookupKey.IsNull() && config.LookupKey.ValueString() != ""

	if err := requireExactlyOneSelector(hasID, hasKey, "id", "lookup_key"); err != nil {
		resp.Diagnostics.AddError("Invalid entitlement selector", err.Error())
		return
	}

	var entitlement *revenuecat.Entitlement
	if hasID {
		found, err := d.client.GetEntitlement(ctx, projectID, config.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Unable to read the RevenueCat entitlement", err.Error())
			return
		}
		entitlement = found
	} else {
		entitlements, err := d.client.ListEntitlements(ctx, projectID)
		if err != nil {
			resp.Diagnostics.AddError("Unable to list RevenueCat entitlements", err.Error())
			return
		}
		found, err := findByKey(entitlements, config.LookupKey.ValueString(),
			func(e revenuecat.Entitlement) string { return e.LookupKey }, "entitlement", "lookup_key")
		if err != nil {
			resp.Diagnostics.AddError("No matching RevenueCat entitlement", err.Error())
			return
		}
		entitlement = found
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, entitlementToModel(projectID, entitlement))...)
}

// --- Offering -------------------------------------------------------------

var (
	_ datasource.DataSource              = (*offeringDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*offeringDataSource)(nil)
)

// NewOfferingDataSource returns the revenuecat_offering data source.
func NewOfferingDataSource() datasource.DataSource {
	return &offeringDataSource{}
}

type offeringDataSource struct {
	client *revenuecat.Client
}

func (d *offeringDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_offering"
}

func (d *offeringDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up an offering in a RevenueCat project by identifier or by lookup " +
			"key. Exactly one of `id` or `lookup_key` must be set.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the project the offering belongs to.",
				Required:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the offering. Conflicts with `lookup_key`.",
				Optional:            true,
				Computed:            true,
			},
			"lookup_key": schema.StringAttribute{
				MarkdownDescription: "Lookup key of the offering. Conflicts with `id`.",
				Optional:            true,
				Computed:            true,
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name of the offering.",
				Computed:            true,
			},
			"is_current": schema.BoolAttribute{
				MarkdownDescription: "Whether this offering is the project's current offering.",
				Computed:            true,
			},
			"metadata": schema.MapAttribute{
				MarkdownDescription: "Arbitrary string key-value pairs attached to the offering.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"created_at": schema.Int64Attribute{
				MarkdownDescription: "Creation time of the offering, in milliseconds since the Unix epoch.",
				Computed:            true,
			},
		},
	}
}

func (d *offeringDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *offeringDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config offeringModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := config.ProjectID.ValueString()
	hasID := !config.ID.IsNull() && config.ID.ValueString() != ""
	hasKey := !config.LookupKey.IsNull() && config.LookupKey.ValueString() != ""

	if err := requireExactlyOneSelector(hasID, hasKey, "id", "lookup_key"); err != nil {
		resp.Diagnostics.AddError("Invalid offering selector", err.Error())
		return
	}

	var offering *revenuecat.Offering
	if hasID {
		found, err := d.client.GetOffering(ctx, projectID, config.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Unable to read the RevenueCat offering", err.Error())
			return
		}
		offering = found
	} else {
		offerings, err := d.client.ListOfferings(ctx, projectID)
		if err != nil {
			resp.Diagnostics.AddError("Unable to list RevenueCat offerings", err.Error())
			return
		}
		found, err := findByKey(offerings, config.LookupKey.ValueString(),
			func(o revenuecat.Offering) string { return o.LookupKey }, "offering", "lookup_key")
		if err != nil {
			resp.Diagnostics.AddError("No matching RevenueCat offering", err.Error())
			return
		}
		offering = found
	}

	// A data source reports what the API holds, so an absent metadata map reads
	// back as null rather than borrowing a prior value.
	state := offeringToModel(ctx, projectID, offering, types.MapNull(types.StringType), &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// --- Package --------------------------------------------------------------

var (
	_ datasource.DataSource              = (*packageDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*packageDataSource)(nil)
)

// NewPackageDataSource returns the revenuecat_package data source.
func NewPackageDataSource() datasource.DataSource {
	return &packageDataSource{}
}

type packageDataSource struct {
	client *revenuecat.Client
}

func (d *packageDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_package"
}

func (d *packageDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a package in a RevenueCat offering by identifier or by lookup " +
			"key. Exactly one of `id` or `lookup_key` must be set.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the project the package belongs to.",
				Required:            true,
			},
			"offering_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the offering the package belongs to. Required when " +
					"looking the package up by `lookup_key`.",
				Required: true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the package. Conflicts with `lookup_key`.",
				Optional:            true,
				Computed:            true,
			},
			"lookup_key": schema.StringAttribute{
				MarkdownDescription: "Lookup key of the package. Conflicts with `id`.",
				Optional:            true,
				Computed:            true,
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name of the package.",
				Computed:            true,
			},
			"position": schema.Int64Attribute{
				MarkdownDescription: "Position of the package within its offering.",
				Computed:            true,
			},
			"created_at": schema.Int64Attribute{
				MarkdownDescription: "Creation time of the package, in milliseconds since the Unix epoch.",
				Computed:            true,
			},
		},
	}
}

func (d *packageDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *packageDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config packageModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := config.ProjectID.ValueString()
	offeringID := config.OfferingID.ValueString()
	hasID := !config.ID.IsNull() && config.ID.ValueString() != ""
	hasKey := !config.LookupKey.IsNull() && config.LookupKey.ValueString() != ""

	if err := requireExactlyOneSelector(hasID, hasKey, "id", "lookup_key"); err != nil {
		resp.Diagnostics.AddError("Invalid package selector", err.Error())
		return
	}

	var pkg *revenuecat.Package
	if hasID {
		found, err := d.client.GetPackage(ctx, projectID, config.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Unable to read the RevenueCat package", err.Error())
			return
		}
		pkg = found
	} else {
		packages, err := d.client.ListPackages(ctx, projectID, offeringID)
		if err != nil {
			resp.Diagnostics.AddError("Unable to list RevenueCat packages", err.Error())
			return
		}
		found, err := findByKey(packages, config.LookupKey.ValueString(),
			func(p revenuecat.Package) string { return p.LookupKey }, "package", "lookup_key")
		if err != nil {
			resp.Diagnostics.AddError("No matching RevenueCat package", err.Error())
			return
		}
		pkg = found
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, packageToModel(projectID, offeringID, pkg))...)
}
