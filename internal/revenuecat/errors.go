package revenuecat

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

// APIError is returned for any non-2xx response from the RevenueCat API. It
// carries the HTTP status alongside the API's own error code and message when
// the response body supplies them, so callers can branch on status rather than
// matching against message text.
type APIError struct {
	// StatusCode is the HTTP status of the response.
	StatusCode int
	// Code is the machine-readable error code from the API, if present.
	Code string
	// Message is the human-readable message from the API, if present.
	Message string
	// Method and Path describe the request that failed.
	Method string
	Path   string
	// Body holds an excerpt of the raw response body when it could not be
	// decoded as a structured API error.
	Body string

	// retryAfter carries a Retry-After hint from the response, which is only
	// readable while the response is in hand. The backoff calculation reads it
	// when deciding how long to wait before the next attempt.
	retryAfter time.Duration
}

func (e *APIError) Error() string {
	detail := e.Message
	if detail == "" {
		detail = e.Body
	}

	switch {
	case e.Code != "" && detail != "":
		return fmt.Sprintf("%s %s: %d %s: %s (code %s)", e.Method, e.Path, e.StatusCode, http.StatusText(e.StatusCode), detail, e.Code)
	case detail != "":
		return fmt.Sprintf("%s %s: %d %s: %s", e.Method, e.Path, e.StatusCode, http.StatusText(e.StatusCode), detail)
	default:
		return fmt.Sprintf("%s %s: %d %s", e.Method, e.Path, e.StatusCode, http.StatusText(e.StatusCode))
	}
}

// IsNotFound reports whether err is an APIError with a 404 status. Resources
// use it to tell "the object is gone, drop it from state" apart from "the
// request failed, raise a diagnostic".
func IsNotFound(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}

// StatusCode returns the HTTP status carried by err, or 0 if err is not an
// APIError.
func StatusCode(err error) int {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode
	}
	return 0
}

// IsRefused reports whether err is the API declining the request as asked: a
// 400, 409 or 422, which says the change itself is not allowed, as opposed to a
// failure to authenticate, find, reach or be served by the API.
func IsRefused(err error) bool {
	switch StatusCode(err) {
	case http.StatusBadRequest, http.StatusConflict, http.StatusUnprocessableEntity:
		return true
	}
	return false
}
