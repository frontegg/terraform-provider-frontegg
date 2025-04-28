# AWS SES example
resource "frontegg_email_providers" "ses_example" {
  provider_name = "ses"
  provider_id   = "AKIAIOSFODNN7EXAMPLE"
  region        = "us-west-2"
  secret        = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
}

# Mailgun example
resource "frontegg_email_providers" "mailgun_example" {
  provider_name = "mailgun"
  domain        = "mail.example.com"
  region        = "us"
  secret        = "key-abcdef123456789"
}

# SendGrid example
resource "frontegg_email_providers" "sendgrid_example" {
  provider_name = "sendgrid"
  secret        = "SG.abcdefghijklmnopqrstuvwxyz"
}

# AWS SES with IAM Role example
resource "frontegg_email_providers" "ses_role_example" {
  provider_name = "ses-role"
  secret        = "arn:aws:iam::123456789012:role/EmailSenderRole"
}
