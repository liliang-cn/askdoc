package domain

import "errors"

var (
	// ErrNotFound indicates resource not found
	ErrNotFound = errors.New("resource not found")
	// ErrInvalidRequest indicates invalid request
	ErrInvalidRequest = errors.New("invalid request")
	// ErrUnauthorized indicates unauthorized access
	ErrUnauthorized = errors.New("unauthorized")
	// ErrRateLimited indicates rate limit exceeded
	ErrRateLimited = errors.New("rate limit exceeded")
)
