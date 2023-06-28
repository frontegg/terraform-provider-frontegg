resource "frontegg_webhook" "example" {
  enabled     = true
  name        = "Example webhook"
  description = "An example of a webhook"
  url         = "https://example.com/webhook"
  secret      = "example-sekret"
  events = [
    "frontegg.user.authenticated"
  ]
}
