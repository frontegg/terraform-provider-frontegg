# iOS: the app identifier is {teamId}.{bundleId}, as it appears in the
# apple-app-site-association file.
resource "frontegg_associated_domain" "ios" {
  platform = "ios"
  app_id   = "ABCDE12345.com.example.app"
}

# Android: register every certificate the app ships under (debug, release,
# Play App Signing).
resource "frontegg_associated_domain" "android" {
  platform     = "android"
  package_name = "com.example.app"

  sha256_cert_fingerprints = [
    "14:6D:E9:83:C5:73:06:50:D8:EE:B9:95:2F:34:FC:64:16:A0:83:42:E6:1D:BE:A8:8A:04:96:B2:3F:CF:44:E5",
  ]
}
