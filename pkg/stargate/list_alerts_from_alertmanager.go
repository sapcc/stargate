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

	"github.com/sapcc/stargate/pkg/alertmanager"
	"github.com/sapcc/stargate/pkg/api"
)

// HandleInternalListAlertsFromAlertmanager handles listing the alerts from the alertmanager.
func (s *Stargate) HandleInternalListAlertsFromAlertmanager(w http.ResponseWriter, r *http.Request) {
	alertList, err := s.alertmanagerClient.ListAlerts(alertmanager.NewDefaultFilter())
	if err != nil {
		s.logger.LogError("error listing alerts from alertmanager", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(api.Error{Code: http.StatusInternalServerError, Message: "error listing alerts from alertmanager"})
		return
	}
	s.respondWithJSON(w, alertList)
	s.logger.LogDebug("responding to request", "handler", "internalListAlertsFromAlertmanager")
}
