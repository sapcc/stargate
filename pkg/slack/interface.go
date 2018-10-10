package slack

import "net/http"

// Receiver ...
type Receiver interface {
	HandleMessage(w http.ResponseWriter, r *http.Request)
}
