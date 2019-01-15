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

package pagerduty

import (
	"fmt"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/pkg/errors"
	"github.com/prometheus/alertmanager/client"
	"github.com/sapcc/stargate/pkg/alert"
	"github.com/sapcc/stargate/pkg/config"
	"github.com/sapcc/stargate/pkg/log"
)

// StatusAcknowledged ...
const (
	StatusAcknowledged = "acknowledged"
	TypeUserReference  = "user_reference"
)

// Client ...
type Client struct {
	logger          log.Logger
	config          config.Config
	pagerdutyClient *pagerduty.Client
}

// NewClient creates a new pagerduty client
func NewClient(config config.Config, logger log.Logger) *Client {
	logger = log.NewLoggerWith(logger, "component", "pagerduty")

	client := pagerduty.NewClient(config.Pagerduty.AuthToken)
	if client == nil {
		logger.LogFatal("unable to create pagerduty client")
	}
	return &Client{
		logger:          logger,
		config:          config,
		pagerdutyClient: client,
	}
}

// AcknowledgeIncident acknowledges a currently firing incident
func (p *Client) AcknowledgeIncident(alert *client.ExtendedAlert, userEmail string) error {
	if userEmail == "" {
		return fmt.Errorf("cannot acknowledge alert '%s' without a mail address", alert.Alert)
	}

	userID, err := p.findUserIDByEmail(userEmail)
	if err != nil {
		return err
	}

	incident, err := p.findIncidentByAlert(alert)
	if err != nil {
		return err
	}

	ackedIncident := acknowledgeIncident(incident, userID)
	p.logger.LogDebug("acknowledged incident",
		"assignments", ackedIncident.Assignments,
		"acknowledgements", ackedIncident.Acknowledgements,
		"status", ackedIncident.Status,
	)

	return p.pagerdutyClient.ManageIncidents(
		userEmail,
		[]pagerduty.Incident{ackedIncident},
	)
}

// findIncident finds triggered incidents in pagerduty by alertname, region
func (p *Client) findIncidentByAlert(extendedAlert *client.ExtendedAlert) (*pagerduty.Incident, error) {
	regionName, err := alert.GetRegionFromExtendedAlert(extendedAlert)
	if err != nil {
		return nil, err
	}

	alertName, err := alert.GetAlertnameFromExtendedAlert(extendedAlert)
	if err != nil {
		return nil, err
	}

	incidentList, err := p.pagerdutyClient.ListIncidents(pagerduty.ListIncidentsOptions{})
	if err != nil {
		return nil, err
	}

	for _, incident := range incidentList.Incidents {
		matchMap, err := parseRegionAndAlertnameFromPagerdutySummary(incident.APIObject.Summary)
		if err != nil {
			p.logger.LogError("incident parsing failed", err)
			continue
		}
		foundAlertname, nameOK := matchMap["alertname"]
		foundRegion, regionOK := matchMap["region"]
		if !nameOK || !regionOK {
			p.logger.LogError(
				"incident parsing failed",
				errors.New("pagerduty incident summary does not contain a region and/or alertname"),
			)
			continue
		}

		p.logger.LogDebug("found incident", "name", foundAlertname, "region", foundRegion)

		if foundAlertname == alertName && foundRegion == regionName {
			return &incident, nil
		}
	}
	return nil, fmt.Errorf("no incident found for alert name: '%s', region: '%s'", alertName, regionName)
}

func (p *Client) findUserIDByEmail(userEmail string) (string, error) {
	userList, err := p.pagerdutyClient.ListUsers(pagerduty.ListUsersOptions{})
	if err != nil {
		return "", errors.Wrapf(err, "failed to list pagerduty users")
	}

	for _, user := range userList.Users {
		if user.Email == userEmail {
			return user.ID, nil
		}
	}

	return "", fmt.Errorf("no pagerduty user with email '%s' found", userEmail)
}

func acknowledgeIncident(incident *pagerduty.Incident, userID string) pagerduty.Incident {
	timeNowUTCString := time.Now().UTC().String()
	ackedIncident := *incident

	ackedIncident.Acknowledgements = append(ackedIncident.Acknowledgements, pagerduty.Acknowledgement{
		At: timeNowUTCString,
		Acknowledger: pagerduty.APIObject{
			Type: TypeUserReference,
			ID:   userID,
		},
	})
	ackedIncident.Assignments = append(ackedIncident.Assignments, pagerduty.Assignment{
		At: timeNowUTCString,
		Assignee: pagerduty.APIObject{
			Type: TypeUserReference,
			ID:   userID,
		},
	})
	ackedIncident.Status = StatusAcknowledged
	return ackedIncident
}
