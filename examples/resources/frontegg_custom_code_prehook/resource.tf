resource "frontegg_custom_code_prehook" "example" {
  enabled     = true
  name        = "Example custom code prehook"
  description = "An example of a custom code prehook"
  runtime     = "NODE_20"
  timeout     = 10
  events = [
    "SIGN_UP"
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
