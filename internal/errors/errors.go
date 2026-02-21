package errors

import "errors"

var ErrInvalidInput = errors.New("invalid input")
var ErrNotFound = errors.New("not found")
var ErrSessionExpired = errors.New("session expired")
