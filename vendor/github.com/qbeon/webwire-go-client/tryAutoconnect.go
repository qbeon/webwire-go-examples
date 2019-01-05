package client

import (
	"context"
	"sync/atomic"

	wwr "github.com/qbeon/webwire-go"
)

// tryAutoconnect tries to connect to the server. If autoconnect is enabled it
// will spawn a new autoconnector goroutine which will periodically poll the
// server and check whether it's available again. If the autoconnector goroutine
// has already been spawned then tryAutoconnect will just await the connection
// or timeout respectively blocking the calling goroutine. If autoconnect is
// temporarily disabled then tryAutoconnect will reactivate it.
//
// ctxHasDeadline should be set to false if the deadline of the context was
// assigned automatically
func (clt *client) tryAutoconnect(
	ctx context.Context,
	ctxHasDeadline bool,
) error {
	// If autoconnect was just temporarily deactivated then reactivate it and
	// try to establish a connection
	atomic.CompareAndSwapInt32(
		&clt.autoconnect,
		autoconnectDeactivated,
		autoconnectEnabled,
	)

	if clt.Status() == StatusConnected {
		return nil
	} else if atomic.LoadInt32(&clt.autoconnect) != autoconnectEnabled {
		// Don't try to auto-connect if it's either temporarily deactivated
		// or completely disabled
		return wwr.ErrDisconnected{}
	}

	// Start the reconnector goroutine if not already started.
	// If it's already started then just proceed to wait until
	// either connected or timed out
	clt.backgroundReconnect()

	// Await flush
	return clt.backReconn.await(ctx, ctxHasDeadline)
}
