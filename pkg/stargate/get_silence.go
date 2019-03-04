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

package stargate

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sapcc/stargate/pkg/api"
)

// HandleGetSilenceByID handles getting the silence by ID.
func (s *Stargate) HandleGetSilenceByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	silenceID, ok := vars["silenceID"]
	if !ok {
		s.logger.LogDebug("not silence ID found in path")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	silence, err := s.alertmanagerClient.GetSilenceByID(silenceID)
	if err != nil {
		s.logger.LogError("error getting silence by id", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(api.Error{Code: http.StatusInternalServerError, Message: "error getting silence by id"})
		return
	}

	s.respondWithJSON(w, silence)
	s.logger.LogDebug("responding to request", "handler", "getSilenceByID")
}
