package gorilla

import (
	"net/http"

	"github.com/qbeon/webwire-go"
)

func (srv *Transport) handleAccept(
	resp http.ResponseWriter,
	req *http.Request,
) {
	// Reject incoming connections during shutdown, pretend the server is
	// temporarily unavailable
	if srv.isShuttingdown() {
		http.Error(resp, "server shutting down", http.StatusServiceUnavailable)
		return
	}

	// Handle OPTION requests
	if req.Method == "OPTIONS" {
		if srv.OnOptions != nil {
			srv.OnOptions(resp, req)
		}
		return
	}

	connectionOptions := webwire.ConnectionOptions{
		Connection:       webwire.Accept,
		ConcurrencyLimit: 0,
		Info: map[int]interface{}{
			0: []byte(req.UserAgent()),
		},
	}
	if srv.BeforeUpgrade != nil {
		connectionOptions = srv.BeforeUpgrade(resp, req)
	}

	// Abort connection establishment if the connection was refused
	if connectionOptions.Connection != webwire.Accept {
		return
	}

	// Copy the user agent string
	conn, err := srv.Upgrader.Upgrade(resp, req, nil)
	if err != nil {
		// Establish connection
		srv.ErrorLog.Print("upgrade failed:", err)
		http.Error(
			resp,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError,
		)
		return
	}

	srv.handleConnection(connectionOptions, conn)
}
