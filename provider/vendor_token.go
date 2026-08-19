package provider

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// vendorIDFromToken extracts the vendorId claim from a Frontegg vendor JWT.
// Several vendor-scoped API routes require it in a frontegg-tenant-id header.
// The token is not verified here; it was just issued by /auth/vendor.
func vendorIDFromToken(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("vendor token is not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode vendor token payload: %w", err)
	}
	var claims struct {
		VendorID string `json:"vendorId"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("failed to parse vendor token claims: %w", err)
	}
	return claims.VendorID, nil
}
