package license

import "errors"

var (
	// ErrNoClient is returned when NewManager is called with a nil client.
	ErrNoClient = errors.New("license: client must not be nil")
)
