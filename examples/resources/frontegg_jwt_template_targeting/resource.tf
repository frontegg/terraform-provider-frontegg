resource "frontegg_jwt_template_targeting" "default" {
  rule {
    condition_logic = "and"
    treatment       = "enterprise-template"

    condition {
      attribute = "tenantId"
      op        = "in_list"
      values    = ["tenant-123", "tenant-456"]
      negate    = false
    }
  }

  rule {
    condition_logic = "and"
    treatment       = "internal-template"

    condition {
      attribute = "userEmail"
      op        = "ends_with"
      values    = ["@example.com"]
      negate    = false
    }
  }
}
