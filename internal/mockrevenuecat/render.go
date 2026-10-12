package mockrevenuecat

import "fmt"

// This file maps between request bodies, stored fields, and API
// representations. It mirrors the JSON contract encoded in
// internal/revenuecat; a correction there needs a matching one here, and the
// end-to-end tests fail loudly if the two drift apart.

// copyFields lifts the named keys from a request body into stored fields,
// skipping keys the caller did not supply so an update leaves them untouched.
func copyFields(body map[string]any, names ...string) map[string]any {
	fields := map[string]any{}
	for _, name := range names {
		if value, ok := body[name]; ok {
			fields[name] = value
		}
	}
	return fields
}

// base renders the attributes every object carries.
func base(obj *object, objectType string) map[string]any {
	rendered := map[string]any{
		"object":     objectType,
		"id":         obj.ID,
		"created_at": obj.CreatedAt,
	}
	if obj.ProjectID != "" {
		rendered["project_id"] = obj.ProjectID
	}
	return rendered
}

// withFields merges stored fields into a rendered object, defaulting any name
// the object does not carry so responses have a stable shape.
func withFields(rendered map[string]any, obj *object, defaults map[string]any) map[string]any {
	for name, fallback := range defaults {
		if value, ok := obj.Fields[name]; ok {
			rendered[name] = value
			continue
		}
		if fallback != nil {
			rendered[name] = fallback
		}
	}
	return rendered
}

func renderProject(obj *object) map[string]any {
	return withFields(base(obj, "project"), obj, map[string]any{"name": ""})
}

func renderGeneric(obj *object) map[string]any {
	rendered := base(obj, "object")
	for name, value := range obj.Fields {
		rendered[name] = value
	}
	return rendered
}

// playCredentialsField is where the mock keeps a Play service account
// credential. It is stored apart from the play_store object, and never
// rendered, for two reasons: the real API never returns the credential, and
// keeping it flat lets an update rotate the key without the store's field-wise
// merge clobbering package_name alongside it.
const playCredentialsField = "_play_service_account_credentials"

// The App Store's secrets are kept the way the Play credential is: flat,
// apart from the app_store object, and never rendered. Each key is stored
// with the id and issuer naming it, as one value, so a test can tell a key
// that was resent from one that was not.
const (
	appStoreSharedSecretField    = "_app_store_shared_secret"
	appStoreSubscriptionKeyField = "_app_store_subscription_key"
	appStoreConnectAPIKeyField   = "_app_store_connect_api_key"
)

func appFields(body map[string]any) map[string]any {
	fields := copyFields(body, "name", "type")
	packageName, credentials := splitPlayStore(body)
	if packageName != nil {
		fields["play_store"] = map[string]any{"package_name": packageName}
	}
	if credentials != nil {
		fields[playCredentialsField] = credentials
	}
	if appStore, ok := body["app_store"].(map[string]any); ok {
		fields["app_store"] = map[string]any{"bundle_id": appStore["bundle_id"]}
		copyAppStoreSecrets(appStore, fields)
	}
	return fields
}

// appUpdateFields is narrower than appFields: an app's type, package name and
// bundle are part of its identity, so an update must not be able to change
// them even if one is sent. Credential rotations, and an App Store app's
// vendor number, are the nested changes allowed.
func appUpdateFields(body map[string]any) map[string]any {
	fields := copyFields(body, "name")
	if _, credentials := splitPlayStore(body); credentials != nil {
		fields[playCredentialsField] = credentials
	}
	if appStore, ok := body["app_store"].(map[string]any); ok {
		copyAppStoreSecrets(appStore, fields)
	}
	return fields
}

// copyAppStoreSecrets lifts what an app_store object carries besides its
// bundle into flat stored fields.
func copyAppStoreSecrets(appStore, fields map[string]any) {
	if secret, ok := appStore["shared_secret"]; ok {
		fields[appStoreSharedSecretField] = secret
	}
	if key, ok := appStore["subscription_private_key"].(string); ok {
		fields[appStoreSubscriptionKeyField] = fmt.Sprintf("%s|%v|%v", key,
			appStore["subscription_key_id"], appStore["subscription_key_issuer"])
	}
	if key, ok := appStore["app_store_connect_api_key"].(string); ok {
		fields[appStoreConnectAPIKeyField] = fmt.Sprintf("%s|%v|%v", key,
			appStore["app_store_connect_api_key_id"], appStore["app_store_connect_api_key_issuer"])
	}
	if vendor, ok := appStore["app_store_connect_vendor_number"]; ok {
		fields["app_store_connect_vendor_number"] = vendor
	}
}

// splitPlayStore separates the two halves of a play_store object: the package
// name, which the API echoes back, and the credential, which it never does.
func splitPlayStore(body map[string]any) (packageName, credentials any) {
	playStore, ok := body["play_store"].(map[string]any)
	if !ok {
		return nil, nil
	}
	return playStore["package_name"], playStore["play_service_account_credentials_json"]
}

func renderApp(obj *object) map[string]any {
	rendered := withFields(base(obj, "app"), obj, map[string]any{
		"name": "",
		"type": "",
	})

	if appStore, ok := obj.Fields["app_store"].(map[string]any); ok {
		// Rebuilt from what the real API returns: the bundle, the vendor
		// number and a configured flag per key — never a secret, an id or an
		// issuer.
		subscriptionKey, _ := obj.Fields[appStoreSubscriptionKeyField].(string)
		apiKey, _ := obj.Fields[appStoreConnectAPIKeyField].(string)
		rendered["app_store"] = map[string]any{
			"bundle_id":                            appStore["bundle_id"],
			"subscription_key_configured":          subscriptionKey != "",
			"app_store_connect_api_key_configured": apiKey != "",
			"app_store_connect_vendor_number":      obj.Fields["app_store_connect_vendor_number"],
		}
	}

	playStore, ok := obj.Fields["play_store"].(map[string]any)
	if !ok {
		return rendered
	}

	// Rebuild rather than pass through, so the credential cannot leak into a
	// response even if a future change starts storing it inside this object.
	credentials, _ := obj.Fields[playCredentialsField].(string)
	rendered["play_store"] = map[string]any{
		"package_name": playStore["package_name"],
		"play_service_account_credentials_configured": credentials != "",
	}
	return rendered
}

// validateAppCreate mirrors the real API's discriminated union: "type" alone
// is not enough, the type-specific object must be present too (for the types
// this provider implements — see revenuecat.ImplementedAppTypes).
func validateAppCreate(body map[string]any) *validationError {
	appType, _ := body["type"].(string)
	if appType != "play_store" && appType != "app_store" {
		return nil
	}
	if _, ok := body[appType]; !ok {
		return &validationError{code: "parameter_error", message: "'" + appType + "' is a required property"}
	}
	return nil
}

// productTitleField is where the mock keeps a product's title. It is never
// rendered, because the real API never returns a title: a provider that reads
// it back from a response would pass against an echoing fake and drift against
// RevenueCat.
const productTitleField = "_title"

func productFields(body map[string]any) map[string]any {
	fields := copyFields(body, "store_identifier", "app_id", "type", "display_name")
	if title, ok := body["title"]; ok {
		fields[productTitleField] = title
	}
	if subscription, ok := body["subscription"].(map[string]any); ok {
		fields["subscription_duration"] = subscription["duration"]
	}
	return fields
}

// productUpdateFields mirrors the update endpoint, which changes a display
// name and nothing that names the product in its store.
func productUpdateFields(body map[string]any) map[string]any {
	return copyFields(body, "display_name")
}

// validateProductUpdate mirrors the real API rejecting a product's identity on
// update: its store identifier and app are fixed once it exists.
func validateProductUpdate(body map[string]any) *validationError {
	for _, name := range []string{"store_identifier", "app_id", "title", "subscription"} {
		if _, ok := body[name]; ok {
			return &validationError{code: "parameter_error", message: "Additional properties are not allowed ('" + name + "' was unexpected)"}
		}
	}
	return nil
}

func renderProduct(obj *object) map[string]any {
	rendered := withFields(base(obj, "product"), obj, map[string]any{
		"store_identifier": "",
		"app_id":           "",
		"type":             "",
		"display_name":     "",
	})
	if obj.Fields["type"] == "subscription" {
		rendered["subscription"] = map[string]any{
			"duration":              obj.Fields["subscription_duration"],
			"grace_period_duration": nil,
			"trial_duration":        nil,
		}
	}
	return rendered
}

func entitlementFields(body map[string]any) map[string]any {
	return copyFields(body, "lookup_key", "display_name")
}

func renderEntitlement(obj *object) map[string]any {
	return withFields(base(obj, "entitlement"), obj, map[string]any{
		"lookup_key":   "",
		"display_name": "",
	})
}

// validateEntitlementCreate mirrors the real API requiring display_name on
// create (unlike most other resources' display_name, which is optional).
func validateEntitlementCreate(body map[string]any) *validationError {
	if _, ok := body["display_name"]; !ok {
		return &validationError{code: "parameter_error", message: "'display_name' is a required property"}
	}
	return nil
}

// validateEntitlementUpdate mirrors the real API's update endpoint accepting
// only display_name — lookup_key is part of an entitlement's identity and
// cannot be changed after creation, so sending it 400s as an unexpected
// property.
func validateEntitlementUpdate(body map[string]any) *validationError {
	if _, ok := body["lookup_key"]; ok {
		return &validationError{code: "parameter_error", message: "Additional properties are not allowed ('lookup_key' was unexpected)"}
	}
	return nil
}

// validateOfferingCreate mirrors the real API rejecting "is_current" on
// create ("Additional properties are not allowed") — an offering can only be
// marked current through a later update, never at creation.
func validateOfferingCreate(body map[string]any) *validationError {
	if _, ok := body["is_current"]; ok {
		return &validationError{code: "parameter_error", message: "Additional properties are not allowed ('is_current' was unexpected)"}
	}
	return nil
}

func offeringFields(body map[string]any) map[string]any {
	fields := copyFields(body, "lookup_key", "display_name", "is_current")

	// Metadata arrives as a JSON object; store it as a string map so it
	// round-trips as the provider's map[string]string.
	if raw, ok := body["metadata"]; ok {
		if nested, ok := raw.(map[string]any); ok {
			metadata := make(map[string]string, len(nested))
			for key, value := range nested {
				if text, ok := value.(string); ok {
					metadata[key] = text
				}
			}
			fields["metadata"] = metadata
		}
	}
	return fields
}

func renderOffering(obj *object) map[string]any {
	rendered := withFields(base(obj, "offering"), obj, map[string]any{
		"lookup_key":   "",
		"display_name": "",
		"is_current":   false,
	})

	if metadata, ok := obj.Fields["metadata"].(map[string]string); ok {
		rendered["metadata"] = metadata
	}
	return rendered
}

func packageFields(body map[string]any) map[string]any {
	return copyFields(body, "lookup_key", "display_name", "position")
}

// validatePackageCreate mirrors the real API requiring display_name on
// create (position stays optional there — only update requires it).
func validatePackageCreate(body map[string]any) *validationError {
	if _, ok := body["display_name"]; !ok {
		return &validationError{code: "parameter_error", message: "'display_name' is a required property"}
	}
	return nil
}

// validatePackageUpdate mirrors the real API's update endpoint: display_name
// and position are both required there (unlike on create), and lookup_key —
// part of a package's identity — is rejected as an unexpected property.
func validatePackageUpdate(body map[string]any) *validationError {
	if _, ok := body["lookup_key"]; ok {
		return &validationError{code: "parameter_error", message: "Additional properties are not allowed ('lookup_key' was unexpected)"}
	}
	if _, ok := body["display_name"]; !ok {
		return &validationError{code: "parameter_error", message: "'display_name' is a required property"}
	}
	if _, ok := body["position"]; !ok {
		return &validationError{code: "parameter_error", message: "'position' is a required property"}
	}
	return nil
}

// packageEligibilityCriteria lists the values the real API accepts for a
// package product attachment's eligibility_criteria.
var packageEligibilityCriteria = map[string]bool{
	"all":             true,
	"google_sdk_lt_6": true,
	"google_sdk_ge_6": true,
}

// validatePackageAttachProducts mirrors the real API requiring
// eligibility_criteria (one of a fixed enum) on every attached product —
// unlike an entitlement's plain product_ids, a package's attach_products body
// is a "products" array of {product_id, eligibility_criteria} objects.
func validatePackageAttachProducts(body map[string]any) *validationError {
	products, ok := body["products"].([]any)
	if !ok {
		return &validationError{code: "parameter_error", message: "'products' is a required property"}
	}
	for _, raw := range products {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		criteria, _ := entry["eligibility_criteria"].(string)
		if criteria == "" {
			return &validationError{code: "parameter_error", message: "'eligibility_criteria' is a required property"}
		}
		if !packageEligibilityCriteria[criteria] {
			return &validationError{code: "parameter_error", message: "eligibility_criteria must be one of 'all', 'google_sdk_lt_6', 'google_sdk_ge_6'"}
		}
	}
	return nil
}

func renderPackage(obj *object) map[string]any {
	rendered := withFields(base(obj, "package"), obj, map[string]any{
		"lookup_key":   "",
		"display_name": "",
		"offering_id":  "",
	})

	// Position is genuinely optional: rendering a default would make an unset
	// position read back as 0 and produce a permanent diff.
	if position, ok := obj.Fields["position"]; ok {
		rendered["position"] = position
	}
	return rendered
}

// attachmentsFromBody reads either shape of attach payload: the entitlement
// form carrying product_ids, and the package form carrying products with
// per-product eligibility criteria.
func attachmentsFromBody(body map[string]any) []attachment {
	var out []attachment

	if raw, ok := body["product_ids"].([]any); ok {
		for _, value := range raw {
			if id, ok := value.(string); ok {
				out = append(out, attachment{ProductID: id})
			}
		}
	}

	if raw, ok := body["products"].([]any); ok {
		for _, value := range raw {
			entry, ok := value.(map[string]any)
			if !ok {
				continue
			}
			id, _ := entry["product_id"].(string)
			if id == "" {
				continue
			}
			criteria, _ := entry["eligibility_criteria"].(string)
			out = append(out, attachment{ProductID: id, EligibilityCriteria: criteria})
		}
	}

	return out
}

func productIDsFromBody(body map[string]any) []string {
	var out []string
	if raw, ok := body["product_ids"].([]any); ok {
		for _, value := range raw {
			if id, ok := value.(string); ok {
				out = append(out, id)
			}
		}
	}
	return out
}

// webhookAuthorizationField is where the mock keeps a webhook's authorization
// header. It is never rendered, because the real API never returns it.
const webhookAuthorizationField = "_authorization_header"

// webhookIntegrationFields serves create and update alike: every field is
// changeable in place, and a null environment, app or event list is stored as
// null, which is how a webhook goes back to "all".
func webhookIntegrationFields(body map[string]any) map[string]any {
	fields := copyFields(body, "name", "url", "environment", "event_types", "app_id")
	if header, ok := body["authorization_header"]; ok {
		fields[webhookAuthorizationField] = header
	}
	return fields
}

// validateWebhookIntegrationCreate mirrors the real API requiring a name and
// a URL.
func validateWebhookIntegrationCreate(body map[string]any) *validationError {
	for _, name := range []string{"name", "url"} {
		if _, ok := body[name]; !ok {
			return &validationError{code: "parameter_error", message: "'" + name + "' is a required property"}
		}
	}
	return nil
}

func renderWebhookIntegration(obj *object) map[string]any {
	rendered := withFields(base(obj, "webhook_integration"), obj, map[string]any{
		"name": "",
		"url":  "",
	})
	// Rendered whether set or not, null included, as the real API does.
	for _, name := range []string{"environment", "event_types", "app_id"} {
		rendered[name] = obj.Fields[name]
	}
	return rendered
}
