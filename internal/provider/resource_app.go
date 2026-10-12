package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nmehlei/terraform-provider-revenuecat/internal/revenuecat"
)

var (
	_ resource.Resource                     = (*appResource)(nil)
	_ resource.ResourceWithConfigure        = (*appResource)(nil)
	_ resource.ResourceWithImportState      = (*appResource)(nil)
	_ resource.ResourceWithConfigValidators = (*appResource)(nil)
	_ resource.ResourceWithValidateConfig   = (*appResource)(nil)
)

// NewAppResource returns the revenuecat_app resource.
func NewAppResource() resource.Resource {
	return &appResource{}
}

type appResource struct {
	client *revenuecat.Client
}

type appModel struct {
	ID          types.String `tfsdk:"id"`
	ProjectID   types.String `tfsdk:"project_id"`
	Name        types.String `tfsdk:"name"`
	Type        types.String `tfsdk:"type"`
	PackageName types.String `tfsdk:"package_name"`
	BundleID    types.String `tfsdk:"bundle_id"`
	CreatedAt   types.Int64  `tfsdk:"created_at"`

	// CredentialsJSONWO is write-only: Terraform leaves it null in both plan
	// and state, so it is only ever readable from the configuration.
	CredentialsJSONWO        types.String `tfsdk:"play_service_account_credentials_json_wo"`
	CredentialsJSONWOVersion types.String `tfsdk:"play_service_account_credentials_json_wo_version"`
	CredentialsConfigured    types.Bool   `tfsdk:"play_service_account_credentials_configured"`

	// The App Store's three secrets follow the same pattern: each is
	// write-only, and a version the practitioner owns is what resends it.
	SharedSecretWO        types.String `tfsdk:"shared_secret_wo"`
	SharedSecretWOVersion types.String `tfsdk:"shared_secret_wo_version"`

	SubscriptionPrivateKeyWO        types.String `tfsdk:"subscription_private_key_wo"`
	SubscriptionPrivateKeyWOVersion types.String `tfsdk:"subscription_private_key_wo_version"`
	SubscriptionKeyID               types.String `tfsdk:"subscription_key_id"`
	SubscriptionKeyIssuer           types.String `tfsdk:"subscription_key_issuer"`
	SubscriptionKeyConfigured       types.Bool   `tfsdk:"subscription_key_configured"`

	AppStoreConnectAPIKeyWO         types.String `tfsdk:"app_store_connect_api_key_wo"`
	AppStoreConnectAPIKeyWOVersion  types.String `tfsdk:"app_store_connect_api_key_wo_version"`
	AppStoreConnectAPIKeyID         types.String `tfsdk:"app_store_connect_api_key_id"`
	AppStoreConnectAPIKeyIssuer     types.String `tfsdk:"app_store_connect_api_key_issuer"`
	AppStoreConnectAPIKeyConfigured types.Bool   `tfsdk:"app_store_connect_api_key_configured"`

	AppStoreConnectVendorNumber types.String `tfsdk:"app_store_connect_vendor_number"`
}

// playStoreAttributes and appStoreAttributes are the attributes that belong
// to one store only. ValidateConfig refuses either set on the other store's
// app, because the API would silently ignore it and the practitioner would
// believe a key was configured that never was.
var (
	playStoreAttributes = []string{
		"package_name",
		"play_service_account_credentials_json_wo",
		"play_service_account_credentials_json_wo_version",
	}
	appStoreAttributes = []string{
		"bundle_id",
		"shared_secret_wo", "shared_secret_wo_version",
		"subscription_private_key_wo", "subscription_private_key_wo_version",
		"subscription_key_id", "subscription_key_issuer",
		"app_store_connect_api_key_wo", "app_store_connect_api_key_wo_version",
		"app_store_connect_api_key_id", "app_store_connect_api_key_issuer",
		"app_store_connect_vendor_number",
	}
)

func (r *appResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app"
}

// writeOnlySecret is the schema of one write-only store secret.
func writeOnlySecret(description string) schema.StringAttribute {
	return schema.StringAttribute{
		MarkdownDescription: description + " Write-only: Terraform sends it but never writes it to state " +
			"or to a plan file, so pair it with an `ephemeral` source to keep it out of every artifact. " +
			"Requires Terraform 1.11 or later, and its `_version` attribute alongside it.",
		Optional:  true,
		WriteOnly: true,
		Sensitive: true,
	}
}

// secretVersion is the schema of the marker that resends a write-only secret.
func secretVersion(secret string) schema.StringAttribute {
	return schema.StringAttribute{
		MarkdownDescription: fmt.Sprintf("Arbitrary version marker for `%s`. Terraform cannot detect a "+
			"change to a value it does not store, so changing this is what tells the provider to send the "+
			"secret again — bump it whenever the secret is rotated.", secret),
		Optional: true,
	}
}

// sentOnly is the schema of a value RevenueCat takes but never returns, so it
// is kept from the configuration.
func sentOnly(description string) schema.StringAttribute {
	return schema.StringAttribute{
		MarkdownDescription: description + " RevenueCat does not return it, so it is read from the " +
			"configuration, and is sent again whenever it or its key's version changes.",
		Optional: true,
	}
}

func (r *appResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a store app within a RevenueCat project: an App Store or a Play Store " +
			"app, with the credentials RevenueCat verifies its purchases with.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "RevenueCat identifier of the app.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the project the app belongs to. Changing this forces a new app.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the app.",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: fmt.Sprintf(
					"Store the app belongs to. One of `%s` — RevenueCat's API nests type-specific "+
						"configuration under a key named after the type, and this provider implements that "+
						"nesting for these. Changing this forces a new app.",
					joinBackticked(revenuecat.ImplementedAppTypes),
				),
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(revenuecat.ImplementedAppTypes...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"package_name": schema.StringAttribute{
				MarkdownDescription: "Play Store package identifier (e.g. `com.example.app`). Required when " +
					"`type` is `play_store`, and refused otherwise. Changing this forces a new app.",
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"bundle_id": schema.StringAttribute{
				MarkdownDescription: "App Store bundle identifier (e.g. `com.example.app`). Required when " +
					"`type` is `app_store`, and refused otherwise. Changing this forces a new app.",
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"created_at": schema.Int64Attribute{
				MarkdownDescription: "Creation time of the app, in milliseconds since the Unix epoch.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64UseStateForUnknown(),
				},
			},

			"play_service_account_credentials_json_wo": schema.StringAttribute{
				MarkdownDescription: "Contents of the Google Cloud service account key file RevenueCat uses to " +
					"verify Play purchases server-side. Write-only: Terraform sends it but never writes it to " +
					"state or to a plan file, so pair it with an `ephemeral` source (for example " +
					"`ephemeral.azurerm_key_vault_secret`) to keep the key out of every artifact.\n\n" +
					"Without this credential RevenueCat cannot acknowledge a Play purchase, and Google " +
					"auto-refunds every unacknowledged purchase — so an app left without it appears to work " +
					"until purchases start silently reversing.\n\n" +
					"Requires Terraform 1.11 or later, and requires " +
					"`play_service_account_credentials_json_wo_version` to be set alongside it.",
				Optional:  true,
				WriteOnly: true,
				Sensitive: true,
			},
			"play_service_account_credentials_json_wo_version": schema.StringAttribute{
				MarkdownDescription: "Arbitrary version marker for the write-only credential. Terraform cannot " +
					"detect a change to a value it does not store, so changing this is what tells the provider " +
					"to send the credential again — bump it whenever the key is rotated. Any string works; a " +
					"date or an incrementing number is conventional.",
				Optional: true,
			},
			"play_service_account_credentials_configured": schema.BoolAttribute{
				MarkdownDescription: "Whether RevenueCat holds Play service account credentials for this app. " +
					"Reported by the API, and the only observable signal that the credential is present — a " +
					"`false` here is the difference between purchases being verified and being refunded. Null " +
					"for an app of another store.",
				Computed: true,
			},

			"shared_secret_wo": writeOnlySecret("The app-specific shared secret (32 characters) from App " +
				"Store Connect, which RevenueCat validates older StoreKit receipts with."),
			"shared_secret_wo_version": secretVersion("shared_secret_wo"),

			"subscription_private_key_wo": writeOnlySecret("Contents of the In-App Purchase key (`.p8`) " +
				"RevenueCat verifies App Store transactions and notifications with. Set together with " +
				"`subscription_key_id` and `subscription_key_issuer`."),
			"subscription_private_key_wo_version": secretVersion("subscription_private_key_wo"),
			"subscription_key_id":                 sentOnly("Key ID of the In-App Purchase key."),
			"subscription_key_issuer":             sentOnly("Issuer ID of the In-App Purchase key."),
			"subscription_key_configured": schema.BoolAttribute{
				MarkdownDescription: "Whether RevenueCat holds an In-App Purchase key for this app. Reported by " +
					"the API, and the only observable signal the key is present. Null for an app of another store.",
				Computed: true,
			},

			"app_store_connect_api_key_wo": writeOnlySecret("Contents of an App Store Connect API key " +
				"(`.p8`), which RevenueCat imports products and prices with. Set together with " +
				"`app_store_connect_api_key_id` and `app_store_connect_api_key_issuer`."),
			"app_store_connect_api_key_wo_version": secretVersion("app_store_connect_api_key_wo"),
			"app_store_connect_api_key_id":         sentOnly("Key ID of the App Store Connect API key."),
			"app_store_connect_api_key_issuer":     sentOnly("Issuer ID of the App Store Connect API key."),
			"app_store_connect_api_key_configured": schema.BoolAttribute{
				MarkdownDescription: "Whether RevenueCat holds an App Store Connect API key for this app. " +
					"Reported by the API. Null for an app of another store.",
				Computed: true,
			},

			"app_store_connect_vendor_number": schema.StringAttribute{
				MarkdownDescription: "The App Store Connect vendor number, which RevenueCat needs for financial " +
					"reports. Changed in place; left out, the app keeps the one it has.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// ConfigValidators rejects a secret with no version, and a key with no id or
// issuer. Terraform cannot diff a write-only value, so a secret without a
// version would accept a rotation and silently never send it — the failure
// would surface as purchases RevenueCat cannot verify rather than as an error.
func (r *appResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.RequiredTogether(
			path.MatchRoot("play_service_account_credentials_json_wo"),
			path.MatchRoot("play_service_account_credentials_json_wo_version"),
		),
		resourcevalidator.RequiredTogether(
			path.MatchRoot("shared_secret_wo"),
			path.MatchRoot("shared_secret_wo_version"),
		),
		resourcevalidator.RequiredTogether(
			path.MatchRoot("subscription_private_key_wo"),
			path.MatchRoot("subscription_private_key_wo_version"),
			path.MatchRoot("subscription_key_id"),
			path.MatchRoot("subscription_key_issuer"),
		),
		resourcevalidator.RequiredTogether(
			path.MatchRoot("app_store_connect_api_key_wo"),
			path.MatchRoot("app_store_connect_api_key_wo_version"),
			path.MatchRoot("app_store_connect_api_key_id"),
			path.MatchRoot("app_store_connect_api_key_issuer"),
		),
	}
}

// ValidateConfig holds each store's attributes to its own type: the one that
// names the app is required, and the other store's are refused.
func (r *appResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var appType types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("type"), &appType)...)
	if resp.Diagnostics.HasError() || appType.IsNull() || appType.IsUnknown() {
		return
	}

	var required string
	var refused []string
	switch appType.ValueString() {
	case revenuecat.AppTypeAppStore:
		required, refused = "bundle_id", playStoreAttributes
	case revenuecat.AppTypePlayStore:
		required, refused = "package_name", appStoreAttributes
	default:
		return
	}

	var value types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(required), &value)...)
	if value.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root(required), "Missing store identifier",
			fmt.Sprintf("`%s` is required when `type` is `%s`.", required, appType.ValueString()))
	}

	for _, name := range refused {
		var other types.String
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(name), &other)...)
		if !other.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root(name), "Attribute of another store",
				fmt.Sprintf("`%s` does not apply when `type` is `%s`.", name, appType.ValueString()))
		}
	}
}

func (r *appResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureResourceClient(req, resp)
}

func (r *appResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config appModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Write-only attributes are null in the plan by design, so the secrets
	// have to be read from the configuration.
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	create := revenuecat.CreateAppRequest{
		Name: plan.Name.ValueString(),
		Type: plan.Type.ValueString(),
	}
	switch plan.Type.ValueString() {
	case revenuecat.AppTypeAppStore:
		create.AppStore = &revenuecat.AppStoreConfig{
			BundleID:                    plan.BundleID.ValueString(),
			SharedSecret:                config.SharedSecretWO.ValueString(),
			SubscriptionPrivateKey:      config.SubscriptionPrivateKeyWO.ValueString(),
			SubscriptionKeyID:           plan.SubscriptionKeyID.ValueString(),
			SubscriptionKeyIssuer:       plan.SubscriptionKeyIssuer.ValueString(),
			AppStoreConnectAPIKey:       config.AppStoreConnectAPIKeyWO.ValueString(),
			AppStoreConnectAPIKeyID:     plan.AppStoreConnectAPIKeyID.ValueString(),
			AppStoreConnectAPIKeyIssuer: plan.AppStoreConnectAPIKeyIssuer.ValueString(),
			AppStoreConnectVendorNumber: optionalString(plan.AppStoreConnectVendorNumber),
		}
	case revenuecat.AppTypePlayStore:
		create.PlayStore = &revenuecat.PlayStoreConfig{
			PackageName:                   plan.PackageName.ValueString(),
			ServiceAccountCredentialsJSON: config.CredentialsJSONWO.ValueString(),
		}
	}

	app, err := r.client.CreateApp(ctx, plan.ProjectID.ValueString(), create)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create the RevenueCat app", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, appToModel(plan.ProjectID.ValueString(), app, plan))...)
}

func (r *appResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state appModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	app, err := r.client.GetApp(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		if revenuecat.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read the RevenueCat app", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, appToModel(state.ProjectID.ValueString(), app, state))...)
}

func (r *appResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, config appModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()
	update := revenuecat.UpdateAppRequest{Name: &name}

	// A secret is resent only when its version moved (or, for a key, the id
	// or issuer naming it changed). Sending one on every update would push it
	// over the wire on an unrelated rename; never sending it would make
	// rotation impossible, because Terraform cannot see that a write-only
	// value changed.
	switch state.Type.ValueString() {
	case revenuecat.AppTypeAppStore:
		appStore := revenuecat.AppStoreConfig{}
		send := false
		if !plan.AppStoreConnectVendorNumber.IsUnknown() && !plan.AppStoreConnectVendorNumber.IsNull() &&
			!plan.AppStoreConnectVendorNumber.Equal(state.AppStoreConnectVendorNumber) {
			appStore.AppStoreConnectVendorNumber = optionalString(plan.AppStoreConnectVendorNumber)
			send = true
		}
		if !plan.SharedSecretWOVersion.Equal(state.SharedSecretWOVersion) && !config.SharedSecretWO.IsNull() {
			appStore.SharedSecret = config.SharedSecretWO.ValueString()
			send = true
		}
		if keyChanged(plan.SubscriptionPrivateKeyWOVersion, plan.SubscriptionKeyID, plan.SubscriptionKeyIssuer,
			state.SubscriptionPrivateKeyWOVersion, state.SubscriptionKeyID, state.SubscriptionKeyIssuer) &&
			!config.SubscriptionPrivateKeyWO.IsNull() {
			appStore.SubscriptionPrivateKey = config.SubscriptionPrivateKeyWO.ValueString()
			appStore.SubscriptionKeyID = plan.SubscriptionKeyID.ValueString()
			appStore.SubscriptionKeyIssuer = plan.SubscriptionKeyIssuer.ValueString()
			send = true
		}
		if keyChanged(plan.AppStoreConnectAPIKeyWOVersion, plan.AppStoreConnectAPIKeyID, plan.AppStoreConnectAPIKeyIssuer,
			state.AppStoreConnectAPIKeyWOVersion, state.AppStoreConnectAPIKeyID, state.AppStoreConnectAPIKeyIssuer) &&
			!config.AppStoreConnectAPIKeyWO.IsNull() {
			appStore.AppStoreConnectAPIKey = config.AppStoreConnectAPIKeyWO.ValueString()
			appStore.AppStoreConnectAPIKeyID = plan.AppStoreConnectAPIKeyID.ValueString()
			appStore.AppStoreConnectAPIKeyIssuer = plan.AppStoreConnectAPIKeyIssuer.ValueString()
			send = true
		}
		if send {
			update.AppStore = &appStore
		}

	case revenuecat.AppTypePlayStore:
		if !plan.CredentialsJSONWOVersion.Equal(state.CredentialsJSONWOVersion) && !config.CredentialsJSONWO.IsNull() {
			update.PlayStore = &revenuecat.PlayStoreConfig{
				ServiceAccountCredentialsJSON: config.CredentialsJSONWO.ValueString(),
			}
		}
	}

	app, err := r.client.UpdateApp(ctx, state.ProjectID.ValueString(), state.ID.ValueString(), update)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update the RevenueCat app", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, appToModel(state.ProjectID.ValueString(), app, plan))...)
}

// keyChanged reports whether a versioned key has to be sent again: its
// version moved, or the id or issuer naming it did.
func keyChanged(version, id, issuer, priorVersion, priorID, priorIssuer types.String) bool {
	return !version.Equal(priorVersion) || !id.Equal(priorID) || !issuer.Equal(priorIssuer)
}

func (r *appResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state appModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteApp(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil && !revenuecat.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete the RevenueCat app", err.Error())
	}
}

func (r *appResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := parseImportID(req.ID, "project_id", "app_id")
	if err != nil {
		importIDError(&resp.Diagnostics, err)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// appToModel maps an API app onto the resource model. The project ID comes
// from prior (the plan or state) because the API does not always echo it
// back, and so does everything RevenueCat takes but never returns: the secret
// versions, which are markers the practitioner owns, and the ids and issuers
// naming the App Store keys.
//
// The write-only secrets themselves are always left null. Terraform requires
// that, and it is the property those attributes exist for.
func appToModel(projectID string, app *revenuecat.App, prior appModel) appModel {
	if app.ProjectID != "" {
		projectID = app.ProjectID
	}
	model := appModel{
		ID:        types.StringValue(app.ID),
		ProjectID: types.StringValue(projectID),
		Name:      types.StringValue(app.Name),
		Type:      types.StringValue(app.Type),
		CreatedAt: types.Int64Value(app.CreatedAt),

		PackageName:           types.StringNull(),
		BundleID:              types.StringNull(),
		CredentialsConfigured: types.BoolNull(),

		CredentialsJSONWO:        types.StringNull(),
		CredentialsJSONWOVersion: prior.CredentialsJSONWOVersion,

		SharedSecretWO:        types.StringNull(),
		SharedSecretWOVersion: prior.SharedSecretWOVersion,

		SubscriptionPrivateKeyWO:        types.StringNull(),
		SubscriptionPrivateKeyWOVersion: prior.SubscriptionPrivateKeyWOVersion,
		SubscriptionKeyID:               prior.SubscriptionKeyID,
		SubscriptionKeyIssuer:           prior.SubscriptionKeyIssuer,
		SubscriptionKeyConfigured:       types.BoolNull(),

		AppStoreConnectAPIKeyWO:         types.StringNull(),
		AppStoreConnectAPIKeyWOVersion:  prior.AppStoreConnectAPIKeyWOVersion,
		AppStoreConnectAPIKeyID:         prior.AppStoreConnectAPIKeyID,
		AppStoreConnectAPIKeyIssuer:     prior.AppStoreConnectAPIKeyIssuer,
		AppStoreConnectAPIKeyConfigured: types.BoolNull(),

		AppStoreConnectVendorNumber: types.StringNull(),
	}
	if app.PlayStore != nil {
		model.PackageName = types.StringValue(app.PlayStore.PackageName)
		model.CredentialsConfigured = types.BoolValue(app.PlayStore.ServiceAccountCredentialsConfigured)
	}
	if app.AppStore != nil {
		model.BundleID = types.StringValue(app.AppStore.BundleID)
		model.SubscriptionKeyConfigured = types.BoolValue(app.AppStore.SubscriptionKeyConfigured)
		model.AppStoreConnectAPIKeyConfigured = types.BoolValue(app.AppStore.AppStoreConnectAPIKeyConfigured)
		if app.AppStore.AppStoreConnectVendorNumber != nil {
			model.AppStoreConnectVendorNumber = types.StringValue(*app.AppStore.AppStoreConnectVendorNumber)
		}
	}
	return model
}
