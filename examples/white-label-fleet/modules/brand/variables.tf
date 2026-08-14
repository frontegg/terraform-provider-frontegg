variable "key" {
  description = "Stable identifier for the brand. Used as the tenant key, so it cannot be changed after the first apply without replacing the tenant."
  type        = string
}

variable "name" {
  description = "Human-readable brand name, shown in the Frontegg portal."
  type        = string
}

variable "auth_host" {
  description = <<-EOT
    The brand's authentication host, without a scheme, e.g. "auth.brand.example.com".

    This host must also appear in the workspace's custom_domains set. The module
    cannot add it there itself: custom_domains is an attribute of the single
    frontegg_workspace resource, so it has to be aggregated across every brand in
    the root configuration. See the README.
  EOT
  type        = string

  validation {
    condition     = !can(regex("^https?://", var.auth_host))
    error_message = "auth_host must be a bare host, without a scheme."
  }
}

variable "app_url" {
  description = "The brand's own application URL, e.g. \"https://app.brand.example.com\"."
  type        = string
}

variable "mobile_bundle_ids" {
  description = <<-EOT
    Bundle identifiers (iOS) and package names (Android) of the mobile apps for this
    brand, keyed by platform. Each entry produces the callback the Frontegg mobile
    SDKs derive from auth_host.

    Leave empty for brands with no mobile app.
  EOT
  type = object({
    ios     = optional(list(string), [])
    android = optional(list(string), [])
  })
  default = {}
}

variable "extra_redirect_uris" {
  description = "Additional redirect URIs to register for this brand, beyond the derived mobile callbacks."
  type        = list(string)
  default     = []
}

variable "metadata" {
  description = "Metadata to attach to the tenant."
  type        = map(string)
  default     = {}
}
