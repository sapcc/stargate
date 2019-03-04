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
	"errors"
	"net/http"

	"github.com/sapcc/stargate/pkg/alertmanager"

	"github.com/sapcc/stargate/pkg/api"
)

// Data ...
type Data struct {
	Alertname      string `json:"alertname"`
	Region         string `json:"region"`
	AcknowledgedBy string `json:"acknowledgedBy"`
}

func (d *Data) validate() error {
	if d.Alertname == "" {
		return errors.New("alertname cannot be empty")
	}
	if d.Region == "" {
		return errors.New("region cannot be empty")
	}
	if d.AcknowledgedBy == "" {
		return errors.New("acknowledgedBy cannot be empty")
	}
	return nil
}

// HandleInternalAcknowledgeAlert handles acknowledging an alert.
func (s *Stargate) HandleInternalAcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	var d struct {
		Data `json:"data"`
	}

	err := json.NewDecoder(r.Body).Decode(&d)
	if err != nil {
		s.logger.LogError("error decoding data", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(api.Error{Code: http.StatusInternalServerError, Message: "error decoding data"})
		return
	}

	if err := d.validate(); err != nil {
		s.logger.LogError("invalid request body", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(api.Error{Code: http.StatusInternalServerError, Message: "error acknowledging alert"})
		return
	}

	f := alertmanager.NewDefaultFilter()
	f.WithAdditionalFilter(map[string]string{"alertname": d.Data.Alertname, "region": d.Data.Region})
	alertList, err := s.alertmanagerClient.ListAlerts(f)
	if err != nil {
		s.logger.LogError("error acknowledging alert", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(api.Error{Code: http.StatusInternalServerError, Message: "error acknowledging alert"})
		return
	}

	err = s.alertStore.AcknowledgeAndSetMultiple(alertList, d.Data.AcknowledgedBy)
	if err != nil {
		s.logger.LogError("error acknowledging alert", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(api.Error{Code: http.StatusInternalServerError, Message: "error acknowledging alert"})
		return
	}

	s.respondWithJSON(w, nil)
	s.logger.LogDebug("responding to request", "handler", "internalAcknowledgeAlert")
}
