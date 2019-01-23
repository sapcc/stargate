package api

import (
	"encoding/json"
	"net/http"

	"github.com/sapcc/stargate/pkg/config"
	"github.com/sapcc/stargate/pkg/log"
)

type authMiddleware struct {
	userName, password string
	authHandler        http.Handler

	logger log.Logger
}

func newAuthMiddleware(cfg config.Config, logger log.Logger) *authMiddleware {
	return &authMiddleware{
		userName: cfg.Slack.UserName,
		password: cfg.Slack.GetValidationToken(),
		logger:   log.NewLoggerWith(logger, "component", "authMiddleware"),
	}
}

func (a *authMiddleware) enforceBasicAuth(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		userName, password, authOK := r.BasicAuth()
		if !authOK {
			a.logger.LogInfo("username and/or password not provided", "method", r.Method, "path", r.URL.Path)
			json.NewEncoder(w).Encode(Error{Code: http.StatusUnauthorized, Message: "user not authorized. provide username and password"})
			return
		}

		if userName != a.userName || password != a.password {
			a.logger.LogInfo("unauthorized request", "method", r.Method, "path", r.URL.Path)
			json.NewEncoder(w).Encode(Error{Code: http.StatusUnauthorized, Message: "user not authorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
