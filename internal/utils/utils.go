package utils

// MaskToken masks sensitive fields for debug output
func MaskToken(token string) string {
	if token == "" {
		return ""
	}
	return "***MASKED***"
}
