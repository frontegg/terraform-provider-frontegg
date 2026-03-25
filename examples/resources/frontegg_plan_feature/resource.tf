resource "frontegg_plan_feature" "example" {
  plan_id = frontegg_plan.example.id
  feature_ids = [
    frontegg_feature.example.id,
  ]
}
