package client

import (
	"fmt"
	"sync"
	"time"

	"github.com/qbeon/webwire-go"
	"github.com/qbeon/webwire-go/message"
	reqman "github.com/qbeon/webwire-go/requestManager"
)

// NewClient creates a new client instance. A connection on this client must be
// established manually even if autoconnect is enabled because NewClient only
// initializes the instance.
func NewClient(
	implementation Implementation,
	options Options,
	transport webwire.ClientTransport,
) (Client, error) {
	if implementation == nil {
		return nil, fmt.Errorf("missing client implementation")
	}

	if transport == nil {
		return nil, fmt.Errorf("missing client transport layer implementation")
	}

	// Prepare configuration
	if err := options.Prepare(); err != nil {
		return nil, err
	}

	// Diactivate autoconnect by default and disable it completely if configured
	autoconnect := autoconnectStatus(autoconnectDeactivated)
	if options.Autoconnect == webwire.Disabled {
		autoconnect = autoconnectDisabled
	}

	// Initialize socket
	conn, err := transport.NewSocket(options.DialingTimeout)
	if err != nil {
		return nil, fmt.Errorf("couldn't initialize socket: %s", err)
	}

	dialingTimer := time.NewTimer(0)
	<-dialingTimer.C

	// Initialize new client
	return &client{
		options:        options,
		impl:           implementation,
		dialingTimer:   dialingTimer,
		autoconnect:    autoconnect,
		statusLock:     &sync.Mutex{},
		status:         StatusDisconnected,
		sessionLock:    sync.RWMutex{},
		session:        nil,
		apiLock:        sync.RWMutex{},
		backReconn:     newDam(),
		connecting:     false,
		connectingLock: sync.RWMutex{},
		connectLock:    sync.Mutex{},
		conn:           conn,
		readerClosing:  make(chan bool, 1),
		heartbeat:      newHeartbeat(conn, options.ErrorLog),
		requestManager: reqman.NewRequestManager(),
		messagePool:    message.NewSyncPool(options.MessageBufferSize, 1024),
	}, nil
}
