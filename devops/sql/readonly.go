package sql

import (
	"regexp"
	"strings"

	"github.com/leganck/go-devops/devops"
)

var (
	commentLine  = regexp.MustCompile(`(?m)--.*?$`)
	commentBlock = regexp.MustCompile(`(?s)/\*.*?\*/`)
	writeStart   = regexp.MustCompile(`(?i)^(insert|update|delete|drop|alter|create|truncate|replace|grant|revoke|call|load|lock|unlock|set|kill|merge|rename|handler|do)\b`)
)

func CheckReadOnly(sqlText string) error {
	s := stripSQLComments(sqlText)
	if strings.TrimSpace(s) == "" {
		return devops.NewError(devops.KindInvalidArgument, "SQL is empty", nil)
	}
	lower := strings.ToLower(s)
	if strings.Contains(lower, " into outfile") || strings.Contains(lower, " into dumpfile") {
		return devops.NewError(devops.KindInvalidArgument, "SQL writes are not allowed", nil)
	}
	for _, stmt := range splitSQL(s) {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if writeStart.MatchString(stmt) {
			return devops.NewError(devops.KindInvalidArgument, "SQL writes are not allowed", nil)
		}
		first := firstWord(stmt)
		switch first {
		case "select", "show", "describe", "desc", "explain", "with", "table":
		default:
			return devops.NewError(devops.KindInvalidArgument, "SQL writes are not allowed", nil)
		}
	}
	return nil
}

func stripSQLComments(s string) string {
	s = commentBlock.ReplaceAllString(s, " ")
	s = commentLine.ReplaceAllString(s, " ")
	return s
}

func splitSQL(s string) []string {
	var parts []string
	var b strings.Builder
	inSingle, inDouble := false, false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
			b.WriteByte(ch)
		case ch == '"' && !inSingle:
			inDouble = !inDouble
			b.WriteByte(ch)
		case ch == ';' && !inSingle && !inDouble:
			parts = append(parts, b.String())
			b.Reset()
		default:
			b.WriteByte(ch)
		}
	}
	if b.Len() > 0 {
		parts = append(parts, b.String())
	}
	return parts
}

func firstWord(s string) string {
	s = strings.TrimSpace(s)
	for i, r := range s {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' || r == '(' {
			return strings.ToLower(s[:i])
		}
	}
	return strings.ToLower(s)
}
