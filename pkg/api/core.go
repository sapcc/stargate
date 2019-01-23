/*******************************************************************************
*
* Copyright 2019 SAP SE
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You should have received a copy of the License along with this
* program. If not, you may obtain a copy of the License at
*
*     http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*
*******************************************************************************/

package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sapcc/stargate/pkg/config"
	"github.com/sapcc/stargate/pkg/log"
	"github.com/sapcc/stargate/pkg/metrics"
)

// API is the Stargate API struct
type API struct {
	*mux.Router
	*authMiddleware
	logger log.Logger

	Config config.Config
}

// NewAPI creates a new API based on the configuration
func NewAPI(config config.Config, logger log.Logger) *API {
	api := &API{
		mux.NewRouter().StrictSlash(false),
		newAuthMiddleware(config, logger),
		log.NewLoggerWith(logger, "component", "api"),
		config,
	}

	api.addRoute("", http.MethodGet, "/", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(NewAPIInfo(config.ExternalURL)); err != nil {
			json.NewEncoder(w).Encode(Error{Code: 500, Message: err.Error()})
		}
	})

	return api
}

// AddRouteV1 adds a new route to the v1 API
func (a *API) AddRouteV1(method, path string, handleFunc func(w http.ResponseWriter, r *http.Request)) {
	a.addRoute("/v1", method, path, handleFunc)
}

// AddRouteV1WithBasicAuth adds a  new route to the v1 API that requires basic auth
func (a *API) AddRouteV1WithBasicAuth(method, path string, handleFunc func(w http.ResponseWriter, r *http.Request)) {
	a.addRoute("/v1", method, path, a.enforceBasicAuth(handleFunc))
}

func (a *API) addRoute(pathPrefix, method, path string, handleFunc func(w http.ResponseWriter, r *http.Request)) {
	a.PathPrefix(pathPrefix).Methods(method).Path(path).HandlerFunc(
		promhttp.InstrumentHandlerCounter(
			metrics.HTTPRequestsTotal.MustCurryWith(prometheus.Labels{"method": method, "handler": pathPrefix + path}),
			http.HandlerFunc(handleFunc)),
	)
}

// Serve starts the stargate API
func (a *API) Serve() error {
	host := fmt.Sprintf("0.0.0.0:%d", a.Config.ListenPort)
	a.logger.LogInfo("starting api", "host", "0.0.0.0", "port", a.Config.ListenPort)
	return http.ListenAndServe(host, a)
}
