package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nmehlei/terraform-provider-revenuecat/internal/revenuecat"
)

var (
	_ resource.Resource                = (*offeringResource)(nil)
	_ resource.ResourceWithConfigure   = (*offeringResource)(nil)
	_ resource.ResourceWithImportState = (*offeringResource)(nil)
)

// NewOfferingResource returns the revenuecat_offering resource.
func NewOfferingResource() resource.Resource {
	return &offeringResource{}
}

type offeringResource struct {
	client *revenuecat.Client
}

type offeringModel struct {
	ID          types.String `tfsdk:"id"`
	ProjectID   types.String `tfsdk:"project_id"`
	LookupKey   types.String `tfsdk:"lookup_key"`
	DisplayName types.String `tfsdk:"display_name"`
	IsCurrent   types.Bool   `tfsdk:"is_current"`
	Metadata    types.Map    `tfsdk:"metadata"`
	CreatedAt   types.Int64  `tfsdk:"created_at"`
}

func (r *offeringResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_offering"
}

func (r *offeringResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an offering within a RevenueCat project. An offering groups the " +
			"packages presented to customers on a paywall.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "RevenueCat identifier of the offering.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the project the offering belongs to. Changing this forces a new offering.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"lookup_key": schema.StringAttribute{
				MarkdownDescription: "Key used to reference the offering from the RevenueCat SDKs, for example `default`.",
				Required:            true,
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name of the offering, shown in the RevenueCat dashboard.",
				Optional:            true,
			},
			"is_current": schema.BoolAttribute{
				MarkdownDescription: "Whether this offering is the project's current offering. Only one " +
					"offering per project can be current; making one current clears the flag on the previous one. " +
					"RevenueCat makes a project's first offering current on its own: when that offering is " +
					"configured `false`, the provider clears the flag after the create, and where RevenueCat " +
					"keeps it or refuses the clear, warns and records `false` — the offering stays current until " +
					"another one is made current, and an update that clears it again fails until then.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"metadata": schema.MapAttribute{
				MarkdownDescription: "Arbitrary string key-value pairs delivered alongside the offering to the SDKs.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"created_at": schema.Int64Attribute{
				MarkdownDescription: "Creation time of the offering, in milliseconds since the Unix epoch.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *offeringResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureResourceClient(req, resp)
}

func (r *offeringResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan offeringModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	metadata := metadataFromFramework(ctx, plan.Metadata, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	offering, err := r.client.CreateOffering(ctx, plan.ProjectID.ValueString(), revenuecat.CreateOfferingRequest{
		LookupKey:   plan.LookupKey.ValueString(),
		DisplayName: optionalString(plan.DisplayName),
		Metadata:    metadata,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create the RevenueCat offering", err.Error())
		return
	}

	// The API rejects is_current on create ("Additional properties are not
	// allowed"), so the flag is set by an update after it. The create decides
	// a value of its own all the same: a project's first offering comes back
	// current. Either way the planned value is the one to reach.
	wantCurrent := plan.IsCurrent.ValueBool()
	if offering.IsCurrent != wantCurrent {
		updated, err := r.client.UpdateOffering(ctx, plan.ProjectID.ValueString(), offering.ID, revenuecat.UpdateOfferingRequest{
			IsCurrent: &wantCurrent,
		})
		switch {
		case err == nil && updated.IsCurrent == wantCurrent:
			offering = updated
		case !wantCurrent && (err == nil || revenuecat.IsRefused(err)):
			// RevenueCat made it current as the project's first offering and
			// keeps it so, by answering true or by refusing the clear. That is
			// the API's own default, not a failed apply: the offering exists as
			// configured in every other respect, and it stops being current once
			// another offering is made current. Record the planned value, as an
			// apply must, and say so; the next refresh reads what RevenueCat
			// holds, and an update that clears it again says what to do.
			detail := "RevenueCat kept it current"
			if err != nil {
				detail = "RevenueCat refused to clear the flag: " + err.Error()
			}
			resp.Diagnostics.AddWarning(
				"RevenueCat made this offering the project's current one",
				fmt.Sprintf("Offering %q was created as the project's first offering, which RevenueCat makes current, and the configuration says is_current = false. %s. It stays current until another offering is made current.", plan.LookupKey.ValueString(), detail),
			)
			offering.IsCurrent = wantCurrent
		default:
			// The flag was not reached, and not because RevenueCat declined it:
			// the update failed, or the API answered something else. The
			// offering exists all the same, so keep it in state as RevenueCat
			// holds it; the error then taints it rather than orphaning it.
			if err == nil {
				err = fmt.Errorf("the API answered is_current=%t", updated.IsCurrent)
			}
			summary := "Unable to clear the RevenueCat offering's current flag"
			if wantCurrent {
				summary = "Unable to mark the RevenueCat offering as current"
			}
			created := offeringToModel(ctx, plan.ProjectID.ValueString(), offering, plan.Metadata, &resp.Diagnostics)
			resp.Diagnostics.Append(resp.State.Set(ctx, created)...)
			resp.Diagnostics.AddError(summary, err.Error())
			return
		}
	}

	state := offeringToModel(ctx, plan.ProjectID.ValueString(), offering, plan.Metadata, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *offeringResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state offeringModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	offering, err := r.client.GetOffering(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		if revenuecat.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read the RevenueCat offering", err.Error())
		return
	}

	refreshed := offeringToModel(ctx, state.ProjectID.ValueString(), offering, state.Metadata, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, refreshed)...)
}

func (r *offeringResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state offeringModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	metadata := metadataFromFramework(ctx, plan.Metadata, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	lookupKey := plan.LookupKey.ValueString()
	offering, err := r.client.UpdateOffering(ctx, state.ProjectID.ValueString(), state.ID.ValueString(), revenuecat.UpdateOfferingRequest{
		LookupKey:   &lookupKey,
		DisplayName: optionalString(plan.DisplayName),
		IsCurrent:   optionalBool(plan.IsCurrent),
		Metadata:    metadata,
	})
	if err != nil {
		detail := err.Error()
		if state.IsCurrent.ValueBool() && !plan.IsCurrent.ValueBool() && revenuecat.IsRefused(err) {
			detail += "\n\nThis update clears is_current on the project's current offering. If that is what RevenueCat refused, it keeps a current offering until another one is made current: make another offering current, or configure this one is_current = true."
		}
		resp.Diagnostics.AddError("Unable to update the RevenueCat offering", detail)
		return
	}

	refreshed := offeringToModel(ctx, state.ProjectID.ValueString(), offering, plan.Metadata, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, refreshed)...)
}

func (r *offeringResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state offeringModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteOffering(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil && !revenuecat.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete the RevenueCat offering", err.Error())
	}
}

func (r *offeringResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := parseImportID(req.ID, "project_id", "offering_id")
	if err != nil {
		importIDError(&resp.Diagnostics, err)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// offeringToModel maps an API offering onto the resource model. priorMetadata
// is the metadata already in plan or state: when the API omits metadata from
// its response, keeping the prior value avoids a spurious diff on a field the
// API simply does not echo back.
func offeringToModel(ctx context.Context, projectID string, offering *revenuecat.Offering, priorMetadata types.Map, diags *diag.Diagnostics) offeringModel {
	if offering.ProjectID != "" {
		projectID = offering.ProjectID
	}

	metadata := metadataToFramework(ctx, offering.Metadata, diags)
	if offering.Metadata == nil {
		metadata = priorMetadata
	}

	return offeringModel{
		ID:          types.StringValue(offering.ID),
		ProjectID:   types.StringValue(projectID),
		LookupKey:   types.StringValue(offering.LookupKey),
		DisplayName: stringOrNull(offering.DisplayName),
		IsCurrent:   types.BoolValue(offering.IsCurrent),
		Metadata:    metadata,
		CreatedAt:   types.Int64Value(offering.CreatedAt),
	}
}
