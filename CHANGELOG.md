# Changelog

## v0.2.36

### Changed

**BREAKING CHANGES:**
Inside Admin Portal changed palette structure from

```tf
 palette {
      success        = "#2ca744"
      info           = "#5587c0"
      warning        = "#ffc107"
      error          = "#e1583e"
      primary        = "#43bb7a"
      primary_text   = "#ffffff"
      secondary      = "#fbfbfc"
      secondary_text = "#3c4a5a"
}
```

to new structure:

````tf
 palette {
      success        = "#2ca744"
      info           = "#5587c0"
      warning        = "#ffc107"
      error          = "#e1583e"
      primary        = "#43bb7a"
      primary_text   = "#ffffff"
      secondary      = "#fbfbfc"
      secondary_text = "#3c4a5a"
}
```palette {
      error {
        contrast_text = "#eeeef0"
        dark= "#ae402c"
        light= "#FFEEEA"
        main= "#E1583E"
      }
      info {
        contrast_text = "#eeeef0"
        dark= "#3c6492"
        light= "#E2EEF9"
        main= "#5587C0"
      }
      primary {
        active = "#278854"
        contrast_text = "#eeeef0"
        dark = "#36A76A"
        hover = "#32A265"
        light = "#A2E1BF"
        main = "#43BB7A"
      }
      secondary {
        active = "#E6ECF4"
        contrast_text = "#eeeef0"
        dark = "#E6ECF4"
        hover = "#F0F3F8"
        light = "#FBFBFC"
        main = "#FBFBFC"
      }
      success {
        contrast_text = "#eeeef0"
        dark = "#1d7c30"
        light = "#E1F5E2"
        main = "#2CA744"
      }
      warning {
        contrast_text = "#eeeef0"
        dark = "#EAE1C2"
        light = "#F9F4E2"
        main = "#A79D7B"
      }
    }
````
