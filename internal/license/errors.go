package license

import "errors"

var (
	ErrInvalidToken     = errors.New("license: invalid token format")
	ErrSignatureInvalid = errors.New("license: signature verification failed")
	ErrTokenExpired     = errors.New("license: token expired")
	ErrInvalidClaims    = errors.New("license: invalid claims")
	ErrFileNotFound     = errors.New("license: license file not found")
)
