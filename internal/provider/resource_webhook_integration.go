package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	_ resource.Resource                     = (*webhookIntegrationResource)(nil)
	_ resource.ResourceWithConfigure        = (*webhookIntegrationResource)(nil)
	_ resource.ResourceWithImportState      = (*webhookIntegrationResource)(nil)
	_ resource.ResourceWithConfigValidators = (*webhookIntegrationResource)(nil)
)

// NewWebhookIntegrationResource returns the revenuecat_webhook_integration resource.
func NewWebhookIntegrationResource() resource.Resource {
	return &webhookIntegrationResource{}
}

type webhookIntegrationResource struct {
	client *revenuecat.Client
}

type webhookIntegrationModel struct {
	ID          types.String `tfsdk:"id"`
	ProjectID   types.String `tfsdk:"project_id"`
	Name        types.String `tfsdk:"name"`
	URL         types.String `tfsdk:"url"`
	Environment types.String `tfsdk:"environment"`
	EventTypes  types.Set    `tfsdk:"event_types"`
	AppID       types.String `tfsdk:"app_id"`
	CreatedAt   types.Int64  `tfsdk:"created_at"`

	// AuthorizationHeaderWO is write-only: Terraform leaves it null in plan
	// and state, so it is only ever readable from the configuration.
	AuthorizationHeaderWO        types.String `tfsdk:"authorization_header_wo"`
	AuthorizationHeaderWOVersion types.String `tfsdk:"authorization_header_wo_version"`
}

func (r *webhookIntegrationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook_integration"
}

func (r *webhookIntegrationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a webhook RevenueCat sends a project's subscription events to: a " +
			"renewal, a cancellation, a purchase whose app closed before it could report it.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "RevenueCat identifier of the webhook.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the project the webhook belongs to. Changing this forces a new webhook.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the webhook.",
				Required:            true,
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "URL RevenueCat sends events to.",
				Required:            true,
			},
			"environment": schema.StringAttribute{
				MarkdownDescription: fmt.Sprintf("Which purchases' events are sent: `%s`. Left out, both.",
					joinBackticked(revenuecat.WebhookEnvironments)),
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf(revenuecat.WebhookEnvironments...),
				},
			},
			"event_types": schema.SetAttribute{
				MarkdownDescription: "Event types sent, by RevenueCat's names (`initial_purchase`, `renewal`, " +
					"`cancellation`, …). Left out, every event.",
				ElementType: types.StringType,
				Optional:    true,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
			},
			"app_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the one app whose events are sent. Left out, every app's.",
				Optional:            true,
			},
			"created_at": schema.Int64Attribute{
				MarkdownDescription: "Creation time of the webhook, in milliseconds since the Unix epoch.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64UseStateForUnknown(),
				},
			},
			"authorization_header_wo": schema.StringAttribute{
				MarkdownDescription: "Value RevenueCat sends as the `Authorization` header of every delivery, " +
					"which the receiving server checks. Write-only: Terraform sends it but never writes it to " +
					"state or to a plan file, and RevenueCat never returns it. Requires Terraform 1.11 or " +
					"later, and `authorization_header_wo_version` alongside it.",
				Optional:  true,
				WriteOnly: true,
				Sensitive: true,
			},
			"authorization_header_wo_version": schema.StringAttribute{
				MarkdownDescription: "Arbitrary version marker for `authorization_header_wo`. Terraform cannot " +
					"detect a change to a value it does not store, so changing this is what sends the header " +
					"again — bump it whenever the value is rotated.",
				Optional: true,
			},
		},
	}
}

// ConfigValidators rejects a header with no version, which would accept a
// rotation and never send it: the receiving server would go on refusing every
// delivery with nothing in a plan to say why.
func (r *webhookIntegrationResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.RequiredTogether(
			path.MatchRoot("authorization_header_wo"),
			path.MatchRoot("authorization_header_wo_version"),
		),
	}
}

func (r *webhookIntegrationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureResourceClient(req, resp)
}

func (r *webhookIntegrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config webhookIntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	webhook, err := r.client.CreateWebhookIntegration(ctx, plan.ProjectID.ValueString(), revenuecat.CreateWebhookIntegrationRequest{
		Name:                plan.Name.ValueString(),
		URL:                 plan.URL.ValueString(),
		AuthorizationHeader: optionalString(config.AuthorizationHeaderWO),
		Environment:         optionalString(plan.Environment),
		EventTypes:          stringSetFromFramework(ctx, plan.EventTypes, &resp.Diagnostics),
		AppID:               optionalString(plan.AppID),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create the RevenueCat webhook", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, webhookToModel(ctx, plan.ProjectID.ValueString(), webhook, plan.AuthorizationHeaderWOVersion, &resp.Diagnostics))...)
}

func (r *webhookIntegrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state webhookIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	webhook, err := r.client.GetWebhookIntegration(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		if revenuecat.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read the RevenueCat webhook", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, webhookToModel(ctx, state.ProjectID.ValueString(), webhook, state.AuthorizationHeaderWOVersion, &resp.Diagnostics))...)
}

func (r *webhookIntegrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, config webhookIntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	update := revenuecat.UpdateWebhookIntegrationRequest{
		Name:        plan.Name.ValueString(),
		URL:         plan.URL.ValueString(),
		Environment: optionalString(plan.Environment),
		EventTypes:  stringSetFromFramework(ctx, plan.EventTypes, &resp.Diagnostics),
		AppID:       optionalString(plan.AppID),
	}
	// The header is resent only when its version moved: sending it on every
	// update would push it over the wire on an unrelated rename, and never
	// sending it would make rotation impossible.
	if !plan.AuthorizationHeaderWOVersion.Equal(state.AuthorizationHeaderWOVersion) && !config.AuthorizationHeaderWO.IsNull() {
		update.AuthorizationHeader = optionalString(config.AuthorizationHeaderWO)
	}

	webhook, err := r.client.UpdateWebhookIntegration(ctx, state.ProjectID.ValueString(), state.ID.ValueString(), update)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update the RevenueCat webhook", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, webhookToModel(ctx, state.ProjectID.ValueString(), webhook, plan.AuthorizationHeaderWOVersion, &resp.Diagnostics))...)
}

func (r *webhookIntegrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state webhookIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteWebhookIntegration(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil && !revenuecat.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete the RevenueCat webhook", err.Error())
	}
}

func (r *webhookIntegrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := parseImportID(req.ID, "project_id", "webhook_integration_id")
	if err != nil {
		importIDError(&resp.Diagnostics, err)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// webhookToModel maps an API webhook onto the resource model. The header's
// version comes from the plan or state: it is a marker the practitioner owns,
// which RevenueCat neither stores nor returns. The header itself is always
// left null, as Terraform requires of a write-only value.
func webhookToModel(ctx context.Context, projectID string, webhook *revenuecat.WebhookIntegration, headerVersion types.String, diags *diag.Diagnostics) webhookIntegrationModel {
	if webhook.ProjectID != "" {
		projectID = webhook.ProjectID
	}
	eventTypes := types.SetNull(types.StringType)
	if len(webhook.EventTypes) > 0 {
		set, setDiags := types.SetValueFrom(ctx, types.StringType, webhook.EventTypes)
		diags.Append(setDiags...)
		eventTypes = set
	}
	model := webhookIntegrationModel{
		ID:          types.StringValue(webhook.ID),
		ProjectID:   types.StringValue(projectID),
		Name:        types.StringValue(webhook.Name),
		URL:         types.StringValue(webhook.URL),
		Environment: types.StringNull(),
		EventTypes:  eventTypes,
		AppID:       types.StringNull(),
		CreatedAt:   types.Int64Value(webhook.CreatedAt),

		AuthorizationHeaderWO:        types.StringNull(),
		AuthorizationHeaderWOVersion: headerVersion,
	}
	// The API reports "both environments" as null and, by its schema, may
	// also as an empty string.
	if webhook.Environment != nil && *webhook.Environment != "" {
		model.Environment = types.StringValue(*webhook.Environment)
	}
	if webhook.AppID != nil {
		model.AppID = types.StringValue(*webhook.AppID)
	}
	return model
}
