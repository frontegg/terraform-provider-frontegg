resource "frontegg_prehook" "api_example" {
  enabled     = true
  name        = "Example API prehook"
  description = "An example of a prehook that sends events to a URL"
  type        = "API"
  url         = "https://example.com/prehook"
  secret      = "example-secret"
  events = [
    "SIGN_UP"
  ]
  fail_method = "CLOSE"
}

resource "frontegg_prehook" "custom_code_example" {
  enabled     = true
  name        = "Example custom code prehook"
  description = "An example of a prehook that runs custom code on Frontegg"
  type        = "CUSTOM_CODE"
  runtime     = "NODE_20"
  timeout     = 10
  events = [
    "USER_INVITE"
  ]
  fail_method = "CLOSE"

  code = <<-EOT
    async function onEvent(eventData) {
      return {
        verdict: 'allow',
      };
    }

    exports.onEvent = onEvent;
  EOT
}
