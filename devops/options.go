package devops

import (
	"net/http"
	"time"
)

// Credentials holds username/password. Never logged or persisted.
type Credentials struct {
	Username string
	Password string
}

type options struct {
	baseURL      string
	creds        Credentials
	httpClient   *http.Client
	transport    http.RoundTripper
	timeout      time.Duration
	store        SessionStore
	logger       Logger
	clock        Clock
	freshLogin   bool
}

// Option configures a Client.
type Option func(*options)

// WithBaseURL sets the DevOps API base URL. Required.
func WithBaseURL(u string) Option {
	return func(o *options) { o.baseURL = u }
}

// WithCredentials sets login credentials. Required.
func WithCredentials(c Credentials) Option {
	return func(o *options) { o.creds = c }
}

// WithHTTPClient injects a full HTTP client (cookie jar is created if missing).
func WithHTTPClient(c *http.Client) Option {
	return func(o *options) { o.httpClient = c }
}

// WithTransport injects a custom RoundTripper on the default client.
func WithTransport(rt http.RoundTripper) Option {
	return func(o *options) { o.transport = rt }
}

// WithTimeout sets the HTTP client timeout (default 60s).
func WithTimeout(d time.Duration) Option {
	return func(o *options) { o.timeout = d }
}

// WithSessionStore injects session persistence. Use NopStore() to disable.
func WithSessionStore(s SessionStore) Option {
	return func(o *options) { o.store = s }
}

// WithLogger injects a logger. Default is no-op.
func WithLogger(l Logger) Option {
	return func(o *options) { o.logger = l }
}

// WithClock injects a clock. Default is time.Now.
func WithClock(c Clock) Option {
	return func(o *options) { o.clock = c }
}

// WithFreshLogin ignores persisted sessions and always logs in.
func WithFreshLogin(v bool) Option {
	return func(o *options) { o.freshLogin = v }
}
