package devops

import (
	"net/url"
	"strings"
)

var sensitiveKeys = []string{"password", "secret", "token", "key", "passwd", "cookie"}

func isSensitiveField(field string) bool {
	lower := strings.ToLower(field)
	for _, s := range sensitiveKeys {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

func maskForm(data url.Values) url.Values {
	masked := make(url.Values, len(data))
	for k, v := range data {
		if isSensitiveField(k) {
			masked[k] = []string{"***MASKED***"}
		} else {
			masked[k] = v
		}
	}
	return masked
}

func sanitizeErrorText(s string) string {
	if s == "" {
		return s
	}
	lower := strings.ToLower(s)
	for _, key := range []string{"password=", "password:", "passwd=", "token=", "cookie="} {
		if i := strings.Index(lower, key); i >= 0 {
			return s[:i] + key[:len(key)-1] + "=***MASKED***"
		}
	}
	return s
}

func containsSecret(s, secret string) bool {
	return secret != "" && strings.Contains(s, secret)
}
