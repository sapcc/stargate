/*******************************************************************************
*
* Copyright 2018 SAP SE
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
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sapcc/stargate/pkg/config"
)

// API is the Stargate API struct
type API struct {
	*mux.Router
	Config config.Config
}

// NewAPI creates a new API based on the configuration
func NewAPI(config config.Config) *API {

	router := mux.NewRouter().StrictSlash(false)

	router.Methods(http.MethodGet).Path("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(NewAPIInfo(config.ExternalURL)); err != nil {
			json.NewEncoder(w).Encode(Error{Code: 500, Message: err.Error()})
		}
	})

	return &API{
		router,
		config,
	}
}

// AddRouteV1 adds a new route to the v1 API
func (a *API) AddRouteV1(method, path string, handleFunc func(w http.ResponseWriter, r *http.Request)) {
	a.PathPrefix("/v1").Methods(method).Path(path).HandlerFunc(handleFunc)
}

// Serve starts the stargate API
func (a *API) Serve() error {
	host := fmt.Sprintf("0.0.0.0:%d", a.Config.ListenPort)
	log.Printf("starting server on %s", host)
	return http.ListenAndServe(host, a)
}
