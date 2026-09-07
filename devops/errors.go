package devops

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

// Kind classifies a stable SDK error.
type Kind string

const (
	KindUnauthorized    Kind = "unauthorized"
	KindForbidden       Kind = "forbidden"
	KindInvalidArgument Kind = "invalid_argument"
	KindNotFound        Kind = "not_found"
	KindVersionNotFound Kind = "version_not_found"
	KindRateLimited     Kind = "rate_limited"
	KindTransient       Kind = "transient"
	KindServer          Kind = "server"
)

// Error is a stable, inspectable SDK error. Messages never include secrets.
type Error struct {
	Kind Kind
	Msg  string
	Err  error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Kind, e.Msg, e.Err)
	}
	if e.Msg == "" {
		return string(e.Kind)
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Msg)
}

func (e *Error) Unwrap() error { return e.Err }

func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok || e == nil || t == nil {
		return false
	}
	return e.Kind == t.Kind
}

var (
	ErrUnauthorized    = &Error{Kind: KindUnauthorized, Msg: "authentication required"}
	ErrForbidden       = &Error{Kind: KindForbidden, Msg: "permission denied"}
	ErrInvalidArgument = &Error{Kind: KindInvalidArgument, Msg: "invalid argument"}
	ErrNotFound        = &Error{Kind: KindNotFound, Msg: "not found"}
	ErrVersionNotFound = &Error{Kind: KindVersionNotFound, Msg: "version not found"}
	ErrRateLimited     = &Error{Kind: KindRateLimited, Msg: "rate limited"}
	ErrTransient       = &Error{Kind: KindTransient, Msg: "transient failure"}
	ErrServer          = &Error{Kind: KindServer, Msg: "server error"}
)

func NewError(kind Kind, msg string, err error) *Error {
	return &Error{Kind: kind, Msg: sanitizeErrorText(msg), Err: err}
}

func wrap(sentinel *Error, msg string, err error) error {
	if sentinel == nil {
		sentinel = ErrServer
	}
	return &Error{Kind: sentinel.Kind, Msg: sanitizeErrorText(msg), Err: err}
}

func invalidArg(msg string) error {
	return wrap(ErrInvalidArgument, msg, nil)
}

func forbidden(msg string) error {
	return wrap(ErrForbidden, msg, nil)
}

func notFound(msg string) error {
	return wrap(ErrNotFound, msg, nil)
}

func versionNotFound(msg string) error {
	return wrap(ErrVersionNotFound, msg, nil)
}

func unauthorized(msg string) error {
	return wrap(ErrUnauthorized, msg, nil)
}

func serverErr(msg string, err error) error {
	return wrap(ErrServer, msg, err)
}

func transientErr(msg string, err error) error {
	return wrap(ErrTransient, msg, err)
}

func classifyStatus(code int, path, msg string) error {
	switch code {
	case 401:
		return unauthorized(fmt.Sprintf("HTTP %d for %s", code, path))
	case 403:
		return unauthorized(fmt.Sprintf("HTTP %d for %s", code, path))
	case 404:
		return notFound(fmt.Sprintf("HTTP %d for %s", code, path))
	case 429:
		return wrap(ErrRateLimited, fmt.Sprintf("HTTP %d for %s", code, path), nil)
	default:
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d for %s", code, path)
		}
		if code >= 500 {
			return transientErr(msg, nil)
		}
		return serverErr(msg, nil)
	}
}

func classifyTransport(err error) error {
	if err == nil {
		return nil
	}
	var ne net.Error
	if errors.As(err, &ne) && (ne.Timeout() || ne.Temporary()) {
		return transientErr("network", err)
	}
	if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrForbidden) ||
		errors.Is(err, ErrInvalidArgument) || errors.Is(err, ErrNotFound) ||
		errors.Is(err, ErrVersionNotFound) || errors.Is(err, ErrRateLimited) ||
		errors.Is(err, ErrTransient) || errors.Is(err, ErrServer) {
		return err
	}
	return transientErr("request failed", err)
}

func looksLikeAuthFailure(msg string) bool {
	lower := strings.ToLower(msg)
	for _, m := range []string{"未登录", "请登录", "重新登录", "登录超时", "unauthorized", "authentication", "sys_user_resource"} {
		if strings.Contains(lower, strings.ToLower(m)) {
			return true
		}
	}
	return false
}

// IsSessionExpiredError reports whether err represents an expired session.
func IsSessionExpiredError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrUnauthorized) {
		return true
	}
	msg := strings.ToLower(err.Error())
	markers := []string{
		"session expired",
		"unauthorized",
		"未登录",
		"请登录",
		"重新登录",
		"登录超时",
		"login page",
		"authentication",
		"sys_user_resource",
		"http 401",
		"http 403",
	}
	for _, m := range markers {
		if strings.Contains(msg, strings.ToLower(m)) {
			return true
		}
	}
	return false
}
