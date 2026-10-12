package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// int64UseStateForUnknown keeps a computed integer attribute stable across
// plans instead of showing it as "(known after apply)" on every change.
func int64UseStateForUnknown() planmodifier.Int64 {
	return int64planmodifier.UseStateForUnknown()
}

// requiresReplaceIfKnown forces a new object when a create-only value changes,
// but only once the value in state is known. RevenueCat never returns some
// create-only values (a product's title), so an imported object holds null for
// them; replacing it just because configuration names the value would destroy
// a live object to learn something it already is.
func requiresReplaceIfKnown() planmodifier.String {
	return stringplanmodifier.RequiresReplaceIf(
		func(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
			resp.RequiresReplace = !req.StateValue.IsNull() && !req.PlanValue.IsUnknown()
		},
		"Changing a known value forces a new object.",
		"Changing a known value forces a new object.",
	)
}

// joinBackticked renders a list of allowed values for a Markdown description.
func joinBackticked(values []string) string {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = v
	}
	return strings.Join(quoted, "`, `")
}
