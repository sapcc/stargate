package api

import (
	"encoding/json"
	"net/http"
)

// RespondWithOK responds with 200
func RespondWithOK(w http.ResponseWriter) {
  w.WriteHeader(http.StatusOK)
}

// RespondWithUnauthorized responds with an appropriate json error
func RespondWithUnauthorized(w http.ResponseWriter) {
  json.NewEncoder(w).Encode(
    Error{
      Code: http.StatusUnauthorized,
      Message: "Unauthorized",
    },
  )
}

// RespondWithError responds with a JSON error
func RespondWithError(code int, message string, w http.ResponseWriter) {
	json.NewEncoder(w).Encode(
		Error{
			Code:    code,
			Message: message,
		},
	)
}
