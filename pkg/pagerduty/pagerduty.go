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

package pagerduty

import (
	"fmt"
	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/common/model"
	"github.com/sapcc/stargate/pkg/config"
	"github.com/sapcc/stargate/pkg/util"
	"log"
	"time"
)

// StatusAcknowledged ...
const (
	StatusAcknowledged = "acknowledged"
	StatusTriggered    = "triggered"
)

// Client ...
type Client struct {
	config config.Config
	pagerdutyClient *pagerduty.Client
}

// NewClient creates a new pagerduty client
func NewClient(config config.Config) *Client {
	client := pagerduty.NewClient(config.PagerdutyConfig.AuthToken)
	if client == nil {
		log.Fatalln("unable to create pagerduty client")
	}
	return &Client{
		config: config,
		pagerdutyClient: client,
	}
}

// AcknowledgeIncident acknowledges a currently firing incident
func (p *Client) AcknowledgeIncident(alert *model.Alert, userEmail string) error {
	if userEmail == "" {
		return fmt.Errorf("cannot acknowledge alert '%s' without a mail address", alert.Name())
	}

	incident, err := p.findIncidentByAlert(alert)
	if err != nil {
		return err
	}

	incident.Acknowledgements = append(incident.Acknowledgements, pagerduty.Acknowledgement{
		At: time.Now().UTC().String(),
	})
	incident.Status = StatusAcknowledged
	return p.pagerdutyClient.ManageIncidents(userEmail, []pagerduty.Incident{*incident})
}

// findIncident finds triggered incidents in pagerduty by alertname, region
func (p *Client) findIncidentByAlert(alert *model.Alert) (*pagerduty.Incident, error) {
	regionName, err := util.GetRegionFromAlert(alert)
	if err != nil {
		return nil, err
	}

	incidentList, err := p.pagerdutyClient.ListIncidents(pagerduty.ListIncidentsOptions{
		Statuses: []string{StatusTriggered},
	})
	if err != nil {
		return nil, err
	}

	for _, incident := range incidentList.Incidents {
		matchMap, err := parseRegionAndAlertnameFromPagerdutySummary(incident.APIObject.Summary)
		if err != nil {
			log.Printf("pagerduty incident summary '%s' does not contain a region and/or alertname", incident.APIObject.Summary)
			continue
		}
		foundAlertname, nameOK := matchMap["alertname"]
		foundRegion, regionOK := matchMap["region"]
		if !nameOK || !regionOK {
			log.Printf("pagerduty incident summary '%s' does not contain a region and/or alertname", incident.APIObject.Summary)
			continue
		}
		if foundAlertname == alert.Name() && foundRegion == regionName {
			return &incident, nil
		}
	}
	return nil, fmt.Errorf("no incident found for alert: %v", alert)
}
