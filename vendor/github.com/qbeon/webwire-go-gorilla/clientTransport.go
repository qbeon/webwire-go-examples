package gorilla

import (
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	wwr "github.com/qbeon/webwire-go"
)

// ClientTransport implements the webwire client transport layer with
// gorilla/websocket
type ClientTransport struct {
	// ServerAddress specifies the server address URL
	ServerAddress url.URL

	// Upgrader specifies the websocket connection upgrader
	Dialer websocket.Dialer
}

// NewSocket implements the ClientTransport interface
func (cltTrans *ClientTransport) NewSocket(
	dialTimeout time.Duration,
) (wwr.ClientSocket, error) {
	// Reset handshake timeout to client-enforced dial timeout
	cltTrans.Dialer.HandshakeTimeout = dialTimeout

	serverAddr := cltTrans.ServerAddress
	if serverAddr.Scheme == "https" {
		serverAddr.Scheme = "wss"
	} else {
		serverAddr.Scheme = "ws"
	}

	sock := &Socket{
		serverAddr: serverAddr,
		connected:  false,
		lock:       &sync.Mutex{},
		readLock:   &sync.Mutex{},
		writeLock:  &sync.Mutex{},
		dialer:     cltTrans.Dialer,
	}

	sock.writerIface = writerInterface{sock: sock}

	return sock, nil
}
