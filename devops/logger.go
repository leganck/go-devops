package devops

import (
	"fmt"
	"strings"
)

// Logger is an injectable, side-effect-free log sink. The SDK never writes
// stdout/stderr itself. The default is a no-op logger.
type Logger interface {
	Debug(msg string, kv ...any)
	Info(msg string, kv ...any)
	Warn(msg string, kv ...any)
	Error(msg string, kv ...any)
}

type nopLogger struct{}

func (nopLogger) Debug(string, ...any) {}
func (nopLogger) Info(string, ...any)  {}
func (nopLogger) Warn(string, ...any)  {}
func (nopLogger) Error(string, ...any) {}

func formatKV(kv []any) string {
	if len(kv) == 0 {
		return ""
	}
	var b strings.Builder
	for i := 0; i+1 < len(kv); i += 2 {
		key := fmt.Sprint(kv[i])
		val := fmt.Sprint(kv[i+1])
		if isSensitiveField(key) {
			val = "***MASKED***"
		}
		b.WriteByte(' ')
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(val)
	}
	if len(kv)%2 == 1 {
		b.WriteString(" extra=")
		b.WriteString(fmt.Sprint(kv[len(kv)-1]))
	}
	return b.String()
}
