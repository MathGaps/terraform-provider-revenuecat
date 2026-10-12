package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestProviderMetadata(t *testing.T) {
	p := New("1.2.3")()

	var resp provider.MetadataResponse
	p.Metadata(context.Background(), provider.MetadataRequest{}, &resp)

	if resp.TypeName != "revenuecat" {
		t.Errorf("TypeName = %q, want revenuecat", resp.TypeName)
	}
	if resp.Version != "1.2.3" {
		t.Errorf("Version = %q, want 1.2.3", resp.Version)
	}
}

func TestProviderSchema(t *testing.T) {
	p := New("test")()

	var resp provider.SchemaResponse
	p.Schema(context.Background(), provider.SchemaRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("provider schema has errors: %v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(context.Background()); diags.HasError() {
		t.Fatalf("provider schema failed validation: %v", diags)
	}

	for _, name := range []string{"api_key", "base_url", "max_retries", "request_timeout_seconds"} {
		attr, ok := resp.Schema.Attributes[name]
		if !ok {
			t.Errorf("provider schema is missing the %q attribute", name)
			continue
		}
		if !attr.IsOptional() {
			t.Errorf("attribute %q must be optional so it can come from the environment", name)
		}
	}

	// The API key is a credential: it must never surface in plan output or logs.
	if !resp.Schema.Attributes["api_key"].IsSensitive() {
		t.Error("api_key must be marked sensitive")
	}
}

// TestProviderRegistersEveryType guards against adding a resource or data
// source implementation and forgetting to register it on the provider.
func TestProviderRegistersEveryType(t *testing.T) {
	ctx := context.Background()
	p := New("test")()

	wantResources := []string{
		"revenuecat_app",
		"revenuecat_product",
		"revenuecat_entitlement",
		"revenuecat_offering",
		"revenuecat_package",
		"revenuecat_entitlement_product_attachment",
		"revenuecat_package_product_attachment",
		"revenuecat_webhook_integration",
	}
	gotResources := map[string]bool{}
	for _, newResource := range p.Resources(ctx) {
		var resp resource.MetadataResponse
		newResource().Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "revenuecat"}, &resp)
		gotResources[resp.TypeName] = true
	}
	if len(gotResources) != len(wantResources) {
		t.Errorf("registered %d resources, want %d", len(gotResources), len(wantResources))
	}
	for _, name := range wantResources {
		if !gotResources[name] {
			t.Errorf("resource %q is not registered on the provider", name)
		}
	}

	wantDataSources := []string{
		"revenuecat_project",
		"revenuecat_projects",
		"revenuecat_app",
		"revenuecat_product",
		"revenuecat_entitlement",
		"revenuecat_offering",
		"revenuecat_package",
	}
	gotDataSources := map[string]bool{}
	for _, newDataSource := range p.DataSources(ctx) {
		var resp datasource.MetadataResponse
		newDataSource().Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "revenuecat"}, &resp)
		gotDataSources[resp.TypeName] = true
	}
	if len(gotDataSources) != len(wantDataSources) {
		t.Errorf("registered %d data sources, want %d", len(gotDataSources), len(wantDataSources))
	}
	for _, name := range wantDataSources {
		if !gotDataSources[name] {
			t.Errorf("data source %q is not registered on the provider", name)
		}
	}
}

// TestResourceSchemasAreValid runs the framework's own schema validation over
// every registered resource, which catches malformed attributes, bad plan
// modifiers and missing element types.
func TestResourceSchemasAreValid(t *testing.T) {
	ctx := context.Background()

	for _, newResource := range New("test")().Resources(ctx) {
		r := newResource()

		var metadata resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "revenuecat"}, &metadata)

		t.Run(metadata.TypeName, func(t *testing.T) {
			var resp resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("schema has errors: %v", resp.Diagnostics)
			}
			if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
				t.Fatalf("schema failed validation: %v", diags)
			}

			if _, ok := resp.Schema.Attributes["id"]; !ok {
				t.Error("schema is missing a computed id attribute")
			}
			if resp.Schema.MarkdownDescription == "" {
				t.Error("schema has no description, so the docs page would be empty")
			}
		})
	}
}

func TestDataSourceSchemasAreValid(t *testing.T) {
	ctx := context.Background()

	for _, newDataSource := range New("test")().DataSources(ctx) {
		d := newDataSource()

		var metadata datasource.MetadataResponse
		d.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "revenuecat"}, &metadata)

		t.Run(metadata.TypeName, func(t *testing.T) {
			var resp datasource.SchemaResponse
			d.Schema(ctx, datasource.SchemaRequest{}, &resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("schema has errors: %v", resp.Diagnostics)
			}
			if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
				t.Fatalf("schema failed validation: %v", diags)
			}
			if resp.Schema.MarkdownDescription == "" {
				t.Error("schema has no description, so the docs page would be empty")
			}
		})
	}
}

// TestIdentityAttributesRequireReplace pins the replacement behavior the specs
// call for, so a later schema edit cannot silently turn an identity change into
// an in-place update.
func TestIdentityAttributesRequireReplace(t *testing.T) {
	ctx := context.Background()

	wantRequiresReplace := map[string][]string{
		"revenuecat_app":                            {"project_id", "type", "package_name"},
		"revenuecat_product":                        {"project_id", "app_id", "store_identifier", "type"},
		"revenuecat_entitlement":                    {"project_id", "lookup_key"},
		"revenuecat_offering":                       {"project_id"},
		"revenuecat_package":                        {"project_id", "offering_id", "lookup_key"},
		"revenuecat_entitlement_product_attachment": {"project_id", "entitlement_id"},
		"revenuecat_package_product_attachment":     {"project_id", "package_id"},
	}

	schemas := map[string]fwresource.Schema{}
	for _, newResource := range New("test")().Resources(ctx) {
		r := newResource()

		var metadata resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "revenuecat"}, &metadata)

		var resp resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &resp)
		schemas[metadata.TypeName] = resp.Schema
	}

	for typeName, attrNames := range wantRequiresReplace {
		s, ok := schemas[typeName]
		if !ok {
			t.Errorf("resource %q was not found", typeName)
			continue
		}

		for _, attrName := range attrNames {
			attr, ok := s.Attributes[attrName].(fwresource.StringAttribute)
			if !ok {
				t.Errorf("%s.%s is not a string attribute", typeName, attrName)
				continue
			}

			if !forcesReplacement(ctx, attr) {
				t.Errorf("%s.%s must force replacement when changed", typeName, attrName)
			}
		}
	}
}

// TestUpdatableAttributesDoNotRequireReplace is the other half: attributes the
// specs say update in place must not carry RequiresReplace.
func TestUpdatableAttributesDoNotRequireReplace(t *testing.T) {
	ctx := context.Background()

	inPlace := map[string][]string{
		"revenuecat_app":         {"name"},
		"revenuecat_entitlement": {"display_name"},
		"revenuecat_offering":    {"lookup_key", "display_name"},
		"revenuecat_package":     {"display_name"},
	}

	for _, newResource := range New("test")().Resources(ctx) {
		r := newResource()

		var metadata resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "revenuecat"}, &metadata)

		attrNames, ok := inPlace[metadata.TypeName]
		if !ok {
			continue
		}

		var resp resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &resp)

		for _, attrName := range attrNames {
			attr, ok := resp.Schema.Attributes[attrName].(fwresource.StringAttribute)
			if !ok {
				t.Errorf("%s.%s is not a string attribute", metadata.TypeName, attrName)
				continue
			}
			if forcesReplacement(ctx, attr) {
				t.Errorf("%s.%s must update in place, but forces replacement", metadata.TypeName, attrName)
			}
		}
	}
}

// TestAttachmentDocsWarnAboutSoleOwnership checks the constraint the provider
// cannot enforce at runtime is at least stated where practitioners will read it.
func TestAttachmentDocsWarnAboutSoleOwnership(t *testing.T) {
	ctx := context.Background()

	for _, newResource := range New("test")().Resources(ctx) {
		r := newResource()

		var metadata resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "revenuecat"}, &metadata)

		if !strings.HasSuffix(metadata.TypeName, "_attachment") {
			continue
		}

		var resp resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &resp)

		if !strings.Contains(resp.Schema.MarkdownDescription, "Only one attachment resource") {
			t.Errorf("%s must document that only one attachment resource may manage the relationship", metadata.TypeName)
		}
	}
}

// forcesReplacement runs an attribute's plan modifiers over a value change and
// reports whether any of them asks for replacement. Asserting the modifier's
// behavior rather than matching its description keeps the test meaningful if
// the framework rewords things.
func forcesReplacement(ctx context.Context, attr fwresource.StringAttribute) bool {
	for _, modifier := range attr.PlanModifiers {
		// RequiresReplace only fires on an update, which it recognizes by both
		// the state and the plan being non-null. A zero value for either reads
		// as a create or a destroy and the modifier does nothing.
		raw := tftypes.NewValue(
			tftypes.Object{AttributeTypes: map[string]tftypes.Type{}},
			map[string]tftypes.Value{},
		)
		req := planmodifier.StringRequest{
			Path:        path.Root("test"),
			StateValue:  types.StringValue("before"),
			PlanValue:   types.StringValue("after"),
			ConfigValue: types.StringValue("after"),
			State:       tfsdk.State{Raw: raw, Schema: fwresource.Schema{}},
			Plan:        tfsdk.Plan{Raw: raw, Schema: fwresource.Schema{}},
			Config:      tfsdk.Config{Raw: raw, Schema: fwresource.Schema{}},
		}
		resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
		modifier.PlanModifyString(ctx, req, resp)
		if resp.RequiresReplace {
			return true
		}
	}
	return false
}
