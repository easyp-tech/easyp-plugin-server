package sdk

import (
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// healthMonitor periodically checks the gRPC connection state
// and triggers a reconnect on transient failures.
type healthMonitor struct {
	conn     *grpc.ClientConn
	interval time.Duration
	stopCh   chan struct{}
}

// start runs a background goroutine that monitors connection health.
// It checks conn.GetState() on each tick and calls conn.Connect()
// when the state is TransientFailure or Shutdown.
func (h *healthMonitor) start() {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			state := h.conn.GetState()
			if state == connectivity.TransientFailure || state == connectivity.Shutdown {
				h.conn.Connect()
			}
		case <-h.stopCh:
			return
		}
	}
}

// stop signals the health monitor goroutine to exit.
func (h *healthMonitor) stop() {
	close(h.stopCh)
}
