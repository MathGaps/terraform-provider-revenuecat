package provider

import (
	"context"
	"fmt"

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
	_ resource.Resource                = (*productResource)(nil)
	_ resource.ResourceWithConfigure   = (*productResource)(nil)
	_ resource.ResourceWithImportState = (*productResource)(nil)
)

// NewProductResource returns the revenuecat_product resource.
func NewProductResource() resource.Resource {
	return &productResource{}
}

type productResource struct {
	client *revenuecat.Client
}

type productModel struct {
	ID              types.String `tfsdk:"id"`
	ProjectID       types.String `tfsdk:"project_id"`
	AppID           types.String `tfsdk:"app_id"`
	StoreIdentifier types.String `tfsdk:"store_identifier"`
	Type            types.String `tfsdk:"type"`
	DisplayName     types.String `tfsdk:"display_name"`
	CreatedAt       types.Int64  `tfsdk:"created_at"`

	Title                types.String `tfsdk:"title"`
	SubscriptionDuration types.String `tfsdk:"subscription_duration"`
}

func (r *productResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_product"
}

func (r *productResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a store product within a RevenueCat project.\n\n" +
			"RevenueCat does not support changing a product's identity, so every attribute other " +
			"than `display_name` forces a new product when changed.\n\n" +
			"A product in a Test Store app is simulated by RevenueCat rather than read from a store, " +
			"so it needs `title`, and a subscription one needs `subscription_duration` to renew.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "RevenueCat identifier of the product.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the project the product belongs to. Changing this forces a new product.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"app_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the app the product belongs to. Changing this forces a new product.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"store_identifier": schema.StringAttribute{
				MarkdownDescription: "Product identifier in the underlying store, for example the App Store " +
					"product ID. For a Play Store subscription this is not the bare subscription ID — " +
					"the API 400s with \"must follow the format 'subscriptionId:basePlanId'\" unless it's " +
					"`<subscription ID>:<base plan ID>`, reflecting that a Play Billing subscription is one " +
					"subscription with multiple base plans, each a separate RevenueCat product. Changing " +
					"this forces a new product.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: fmt.Sprintf(
					"Type of the product. One of `%s`. Changing this forces a new product.",
					joinBackticked(revenuecat.ProductTypes),
				),
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(revenuecat.ProductTypes...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name of the product, shown in the RevenueCat dashboard. " +
					"Changed in place. Left out, the product keeps the name it has.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"title": schema.StringAttribute{
				MarkdownDescription: "User-facing title of a Test Store product, which RevenueCat requires " +
					"to create one; ignored for every other store. RevenueCat never returns it, so it is " +
					"read from the configuration: changing a title this provider created forces a new " +
					"product, and an imported product takes the configured title without one.",
				Optional: true,
				PlanModifiers: []planmodifier.String{
					requiresReplaceIfKnown(),
				},
			},
			"subscription_duration": schema.StringAttribute{
				MarkdownDescription: fmt.Sprintf(
					"Billing period of a Test Store subscription product, in ISO-8601: one of `%s`. "+
						"RevenueCat simulates renewals on it, and ignores it for every other store. "+
						"Read back from RevenueCat, which reports it for products of other stores too. "+
						"Changing it forces a new product.",
					joinBackticked(revenuecat.SubscriptionDurations),
				),
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(revenuecat.SubscriptionDurations...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					requiresReplaceIfKnown(),
				},
			},
			"created_at": schema.Int64Attribute{
				MarkdownDescription: "Creation time of the product, in milliseconds since the Unix epoch.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *productResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureResourceClient(req, resp)
}

func (r *productResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan productModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	create := revenuecat.CreateProductRequest{
		StoreIdentifier: plan.StoreIdentifier.ValueString(),
		AppID:           plan.AppID.ValueString(),
		Type:            plan.Type.ValueString(),
		DisplayName:     optionalString(plan.DisplayName),
		Title:           optionalString(plan.Title),
	}
	if duration := optionalString(plan.SubscriptionDuration); duration != nil {
		create.Subscription = &revenuecat.ProductSubscriptionInput{Duration: *duration}
	}

	product, err := r.client.CreateProduct(ctx, plan.ProjectID.ValueString(), create)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create the RevenueCat product", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, productToModel(plan.ProjectID.ValueString(), product, plan.Title))...)
}

func (r *productResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state productModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	product, err := r.client.GetProduct(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		if revenuecat.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read the RevenueCat product", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, productToModel(state.ProjectID.ValueString(), product, state.Title))...)
}

// Update handles the in-place changes: a display name, which RevenueCat's
// update endpoint takes, and a title arriving on a product whose title was
// unknown (an imported one), which RevenueCat cannot change and so is only
// recorded. Every other attribute forces a new product.
func (r *productResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state productModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var (
		product *revenuecat.Product
		err     error
	)
	if !plan.DisplayName.IsUnknown() && !plan.DisplayName.IsNull() && !plan.DisplayName.Equal(state.DisplayName) {
		product, err = r.client.UpdateProduct(ctx, state.ProjectID.ValueString(), state.ID.ValueString(),
			revenuecat.UpdateProductRequest{DisplayName: plan.DisplayName.ValueString()})
	} else {
		product, err = r.client.GetProduct(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to update the RevenueCat product", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, productToModel(state.ProjectID.ValueString(), product, plan.Title))...)
}

func (r *productResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state productModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteProduct(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil && !revenuecat.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete the RevenueCat product", err.Error())
	}
}

func (r *productResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := parseImportID(req.ID, "project_id", "product_id")
	if err != nil {
		importIDError(&resp.Diagnostics, err)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// productToModel maps an API product onto the resource model. The title comes
// from configuration or state, because RevenueCat never returns it.
func productToModel(projectID string, product *revenuecat.Product, title types.String) productModel {
	duration := types.StringNull()
	if product.Subscription != nil && product.Subscription.Duration != nil {
		duration = types.StringValue(*product.Subscription.Duration)
	}
	return productModel{
		ID:              types.StringValue(product.ID),
		ProjectID:       types.StringValue(projectID),
		AppID:           types.StringValue(product.AppID),
		StoreIdentifier: types.StringValue(product.StoreIdentifier),
		Type:            types.StringValue(product.Type),
		DisplayName:     stringOrNull(product.DisplayName),
		CreatedAt:       types.Int64Value(product.CreatedAt),

		Title:                title,
		SubscriptionDuration: duration,
	}
}
