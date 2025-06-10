resource "frontegg_plan" "example" {
  name                    = "Example Plan"
  description             = "An example plan"
  default_treatment       = "true"
  default_time_limitation = 30
  assign_on_signup        = true

  feature_keys = [
    "feature-1",
    "feature-2"
  ]

  rules = [
    {
      "key"      = "tenant.region"
      "operator" = "equals"
      "value"    = "us-west"
    }
  ]
}
