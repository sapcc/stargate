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
	"net/http"

	"github.com/sapcc/stargate/pkg/alert"
	"github.com/sapcc/stargate/pkg/alertmanager"
	"github.com/sapcc/stargate/pkg/store"
)

// HandleListAlerts handles alert listing.
func (s *Stargate) HandleListAlerts(w http.ResponseWriter, r *http.Request) {
	// get a fresh list of alerts from the alertmanager
	filter := alertmanager.NewFilterFromRequest(r)
	alertList, err := s.alertmanagerClient.ListAlerts(filter)
	if err != nil {
		s.logger.LogError("error listing alerts", err)
	}

	// If an alert is also found in the internal alert store by its fingerprint,
	// its annotations will be replaced with the ones from the alert store.
	for idx, extendedAlert := range alertList {
		a, err := s.alertStore.GetFromFingerPrintString(extendedAlert.Fingerprint)
		if err != nil {
			if !store.IsErrNotFound(err) {
				s.logger.LogError("error getting alert from store", err, "alertFingerPrint", extendedAlert.Fingerprint)
			}
			continue
		}
		alertList[idx].Annotations = alert.MergeAnnotations(a, extendedAlert)
	}

	s.respondWithJSON(w, alertList)
	s.logger.LogDebug("responding to request", "handler", "listAlerts")
}
