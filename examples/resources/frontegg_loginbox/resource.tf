resource "frontegg_loginbox" "example" {
  login {
    disclaimer {
      terms {
        enabled = true
      }

      privacy {
        enabled = true
      }
    }
  }

  signup {
    disclaimer {
      terms {
        enabled = true
      }

      privacy {
        enabled = true
      }
    }
  }

  palette {
    primary {
      active        = "#278854"
      contrast_text = "#eeeef0"
      dark          = "#36A76A"
      hover         = "#32A265"
      light         = "#A2E1BF"
      main          = "#43BB7A"
    }
    secondary {
      active        = "#E6ECF4"
      contrast_text = "#eeeef0"
      dark          = "#E6ECF4"
      hover         = "#F0F3F8"
      light         = "#FBFBFC"
      main          = "#FBFBFC"
    }
  }

  theme_name = "modern"
  tenant_id  = "your_tenant_id"

  social_logins {
    social_logins_layout {
      main_button = "google"
    }
  }

  activate_account {
    disclaimer {
      terms {
        enabled = true
      }

      privacy {
        enabled = true
      }
    }
  }
}
