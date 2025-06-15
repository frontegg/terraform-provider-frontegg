resource "frontegg_email_template" "reset_password" {
  template_type = "ResetPassword"
  active        = true
  from_name     = "Your Company"
  from_address  = "noreply@yourcompany.com"
  subject       = "Reset Your Company Password"
  html_template = "<strong>Reset your password! <a href='{{redirectURL}}'>Click here</a></strong>"
  redirect_url  = "https://yourcompany.com/reset"
}

resource "frontegg_email_template" "user_activation" {
  template_type        = "ActivateUser"
  active               = true
  from_name            = "Your Company"
  from_address         = "noreply@yourcompany.com"
  subject              = "Activate your account"
  html_template        = "<h1>Welcome to Your Company!</h1><p>Please <a href='{{redirectURL}}'>click here</a> to activate your account.</p>"
  redirect_url         = "https://yourcompany.com/activate"
  success_redirect_url = "https://yourcompany.com/activated"
}
