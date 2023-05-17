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
      active       = "#278854"
      contrastText = "#eeeef0"
      dark         = "#36A76A"
      hover        = "#32A265"
      light        = "#A2E1BF"
      main         = "#43BB7A"
    }
    secondary {
      active       = "#E6ECF4"
      contrastText = "#eeeef0"
      dark         = "#E6ECF4"
      hover        = "#F0F3F8"
      light        = "#FBFBFC"
      main         = "#FBFBFC"
    }
  }

  themeName = "modern"
  tenantId  = "your_tenant_id"

  socialLogins {
    socialLoginsLayout {
      mainButton = "google"
    }
  }

  activateAccount {
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
