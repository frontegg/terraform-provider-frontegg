package provider

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

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
