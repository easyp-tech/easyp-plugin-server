package license

import "errors"

// ErrNoClient is returned when NewManager is called with a nil client.
var ErrNoClient = errors.New("license: client must not be nil")
