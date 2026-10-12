# An end-to-end RevenueCat catalog: a store app, its products, an entitlement
# granted by those products, and an offering whose packages present them on a
# paywall.
#
# Only Play Store apps today: the API nests type-specific configuration under
# a key named after the store type, and this provider only implements that
# nesting for "play_store" so far (see revenuecat_app's docs).

terraform {
  required_providers {
    revenuecat = {
      source  = "mathgaps/revenuecat"
      version = "~> 0.1"
    }
  }
}

provider "revenuecat" {
  # Reads REVENUECAT_API_KEY from the environment.
}

variable "project_name" {
  description = "Name of the existing RevenueCat project to configure."
  type        = string
  default     = "Acme"
}

# Projects are created in the RevenueCat dashboard, so this configuration adopts
# an existing one rather than managing it.
data "revenuecat_project" "main" {
  name = var.project_name
}

resource "revenuecat_app" "android" {
  project_id   = data.revenuecat_project.main.id
  name         = "Acme Android"
  type         = "play_store"
  package_name = "com.acme.app"
}

resource "revenuecat_product" "monthly" {
  project_id       = data.revenuecat_project.main.id
  app_id           = revenuecat_app.android.id
  store_identifier = "com.acme.pro.monthly"
  type             = "subscription"
  display_name     = "Pro Monthly"
}

resource "revenuecat_product" "annual" {
  project_id       = data.revenuecat_project.main.id
  app_id           = revenuecat_app.android.id
  store_identifier = "com.acme.pro.annual"
  type             = "subscription"
  display_name     = "Pro Annual"
}

resource "revenuecat_entitlement" "pro" {
  project_id   = data.revenuecat_project.main.id
  lookup_key   = "pro"
  display_name = "Pro"
}

# One attachment resource owns every product granting the entitlement. Splitting
# these across two resources would make them fight over the same set.
resource "revenuecat_entitlement_product_attachment" "pro" {
  project_id     = data.revenuecat_project.main.id
  entitlement_id = revenuecat_entitlement.pro.id

  product_ids = [
    revenuecat_product.monthly.id,
    revenuecat_product.annual.id,
  ]
}

resource "revenuecat_offering" "default" {
  project_id   = data.revenuecat_project.main.id
  lookup_key   = "default"
  display_name = "Default Offering"
  is_current   = true

  metadata = {
    paywall_variant = "a"
  }
}

resource "revenuecat_package" "monthly" {
  project_id   = data.revenuecat_project.main.id
  offering_id  = revenuecat_offering.default.id
  lookup_key   = "$rc_monthly"
  display_name = "Monthly"
  position     = 1
}

resource "revenuecat_package" "annual" {
  project_id   = data.revenuecat_project.main.id
  offering_id  = revenuecat_offering.default.id
  lookup_key   = "$rc_annual"
  display_name = "Annual"
  position     = 2
}

resource "revenuecat_package_product_attachment" "monthly" {
  project_id = data.revenuecat_project.main.id
  package_id = revenuecat_package.monthly.id

  product {
    product_id = revenuecat_product.monthly.id
  }
}

resource "revenuecat_package_product_attachment" "annual" {
  project_id = data.revenuecat_project.main.id
  package_id = revenuecat_package.annual.id

  product {
    product_id = revenuecat_product.annual.id
  }
}

output "entitlement_id" {
  description = "Identifier of the managed entitlement."
  value       = revenuecat_entitlement.pro.id
}

output "current_offering_id" {
  description = "Identifier of the offering marked current."
  value       = revenuecat_offering.default.id
}
