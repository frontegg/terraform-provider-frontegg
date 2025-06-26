resource "frontegg_prehook" "example" {
  enabled     = true
  name        = "Example prehook"
  description = "An example of a prehook"
  url         = "https://example.com/prehook"
  secret      = "example-secret"
  events = [
    "SIGN_UP"
  ]
  fail_method = "CLOSE"
}
