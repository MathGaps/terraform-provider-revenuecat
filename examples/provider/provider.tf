terraform {
  required_providers {
    revenuecat = {
      source  = "mathgaps/revenuecat"
      version = "~> 0.1"
    }
  }
}

variable "revenuecat_api_key" {
  description = "A RevenueCat API v2 secret key. Prefer setting REVENUECAT_API_KEY instead."
  type        = string
  sensitive   = true
  default     = null
}

provider "revenuecat" {
  # Leaving api_key unset falls back to the REVENUECAT_API_KEY environment
  # variable, which keeps the credential out of the configuration entirely.
  api_key = var.revenuecat_api_key
}
