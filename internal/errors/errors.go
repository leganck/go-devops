package errors

import (
	"fmt"
)

// PluginError represents a custom error type for the plugin
// It contains additional context about the error

type PluginError struct {
	Code    string
	Message string
	Err     error
}

// Error implements the error interface for PluginError
func (e *PluginError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap implements the error unwrapping interface
func (e *PluginError) Unwrap() error {
	return e.Err
}

// NewPluginError creates a new PluginError
func NewPluginError(code, message string, err error) *PluginError {
	return &PluginError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// ValidationError represents a validation error
func NewValidationError(message string, err error) *PluginError {
	return NewPluginError("VALIDATION_ERROR", message, err)
}

// LoginError represents a login error
func NewLoginError(message string, err error) *PluginError {
	return NewPluginError("LOGIN_ERROR", message, err)
}

// APIError represents an API error
func NewAPIError(message string, err error) *PluginError {
	return NewPluginError("API_ERROR", message, err)
}

// DeploymentError represents a deployment error
func NewDeploymentError(message string, err error) *PluginError {
	return NewPluginError("DEPLOYMENT_ERROR", message, err)
}

// VersionError represents a version error
func NewVersionError(message string, err error) *PluginError {
	return NewPluginError("VERSION_ERROR", message, err)
}

// ServerError represents a server error
func NewServerError(message string, err error) *PluginError {
	return NewPluginError("SERVER_ERROR", message, err)
}
