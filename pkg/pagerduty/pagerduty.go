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
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/alertmanager/client"
	"github.com/sapcc/go-pagerduty"
	"github.com/sapcc/stargate/pkg/alert"
	"github.com/sapcc/stargate/pkg/config"
	"github.com/sapcc/stargate/pkg/log"
)

const (
	// StatusAcknowledged ...
	StatusAcknowledged = "acknowledged"
	// StatusTriggered ...
	StatusTriggered = "triggered"
	// TypeUserReference ...
	TypeUserReference = "user_reference"
)

// ErrUserNotFound is the error raised when a user was not found by its mail address in Pagerduty.
var ErrUserNotFound = errors.New("user not found")

// Client ...
type Client struct {
	logger          log.Logger
	config          config.Config
	pagerdutyClient *pagerduty.Client
	defaultUser     *pagerduty.User
}

// ShortPagerdutyIncident ...
type ShortPagerdutyIncident struct {
	Name   string `json:"name"`
	Region string `json:"region"`
}

// NewClient creates a new pagerduty client.
func NewClient(config config.Config, logger log.Logger) *Client {
	logger = log.NewLoggerWith(logger, "component", "pagerduty")

	pagerdutyClient := pagerduty.NewClient(config.Pagerduty.AuthToken)
	if pagerdutyClient == nil {
		logger.LogFatal("unable to create pagerduty client")
	}
	client := &Client{
		logger:          logger,
		config:          config,
		pagerdutyClient: pagerdutyClient,
	}

	// fallback to default user.
	defaultUserEmail := client.config.Pagerduty.DefaultUserEmail
	if client.defaultUser == nil && defaultUserEmail != "" {
		defaultUser, err := client.findUserIDByEmail(defaultUserEmail)
		if err != nil {
			logger.LogError("unable to get fallback user", err)
		}
		client.defaultUser = defaultUser
	}

	return client
}

// AcknowledgeIncident acknowledges a currently firing incident.
func (p *Client) AcknowledgeIncident(alert *client.ExtendedAlert, userEmail string) error {
	if userEmail == "" {
		return fmt.Errorf("cannot acknowledge alert '%s' without a mail address", alert.Alert)
	}

	incident, err := p.findIncidentByAlert(alert)
	if err != nil {
		return err
	}

	// Attempt to find Pagerduty user by email address.
	user, err := p.findUserIDByEmail(userEmail)
	if err != nil {
		// Return here if there's an error that is not UserNotFound.
		if !isUserNotFound(err) {
			return err
		}

		// Getting here means, we didn't find the user in Pagerduty.
		// Use the default user instead.
		user = p.defaultUser
		p.logger.LogInfo("pagerduty user not found. falling back to default user", "userMail", userEmail, "defaultUserMail", user.Email, "defaultUserID", user.ID)

		if err := p.addActualAcknowledgerAsNoteToIncident(incident, userEmail); err != nil {
			p.logger.LogError("failed to add note to incident", err, "incidentID", incident.ID)
		}
	}

	ackedIncident := acknowledgeIncident(incident, user)
	p.logger.LogDebug("acknowledge incident",
		"incidentID", ackedIncident.Id,
		"assignments", assignmentsToString(ackedIncident.Assignments),
		"acknowledgements", acknowledgementsToString(ackedIncident.Acknowledgements),
		"status", ackedIncident.Status,
	)

	return p.pagerdutyClient.ManageIncidents(
		user.Email,
		[]pagerduty.Incident{ackedIncident},
	)
}

// ListParsedIncidents returns a list of parsed Pagerduty incidents or an error.
func (p *Client) ListParsedIncidents() ([]*ShortPagerdutyIncident, error) {
	incidentList, err := p.listIncidents()
	if err != nil {
		return nil, err
	}

	shortPagerdutyIncidentList := make([]*ShortPagerdutyIncident, 0)
	for _, incident := range incidentList {
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

		shortPagerdutyIncidentList = append(
			shortPagerdutyIncidentList,
			&ShortPagerdutyIncident{Name: foundAlertname, Region: foundRegion},
		)
	}
	return shortPagerdutyIncidentList, nil
}

// findIncident finds triggered incidents in pagerduty by alertname, region.
func (p *Client) findIncidentByAlert(extendedAlert *client.ExtendedAlert) (*pagerduty.Incident, error) {
	regionName, err := alert.GetRegionFromExtendedAlert(extendedAlert)
	if err != nil {
		return nil, err
	}

	alertName, err := alert.GetAlertnameFromExtendedAlert(extendedAlert)
	if err != nil {
		return nil, err
	}

	incidentList, err := p.listIncidents()
	if err != nil {
		return nil, err
	}

	var incidentDebugList []string
	for _, incident := range incidentList {
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

		// for debugging purposed only: used for printing a the list of known incidents in a more comprehensive format
		incidentDebugList = append(incidentDebugList, fmt.Sprintf("[name=%s,region=%s]", foundAlertname, foundRegion))

		if foundAlertname == alertName && foundRegion == regionName {
			return &incident, nil
		}
	}

	p.logger.LogDebug("found incidents", "incidents", strings.Join(incidentDebugList, ", "))
	return nil, fmt.Errorf("no incident found for alert name: '%s', region: '%s'", alertName, regionName)
}

func (p *Client) findUserIDByEmail(userEmail string) (*pagerduty.User, error) {
	userList, err := p.pagerdutyClient.ListUsers(pagerduty.ListUsersOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list pagerduty users")
	}

	for _, user := range userList.Users {
		if user.Email == userEmail {
			return &user, nil
		}
	}

	return nil, ErrUserNotFound
}

func (p *Client) listIncidents() ([]pagerduty.Incident, error) {
	incidentList, err := p.pagerdutyClient.ListIncidents(pagerduty.ListIncidentsOptions{Statuses: []string{StatusTriggered}})
	if err != nil {
		return nil, err
	}
	return incidentList.Incidents, nil
}

func (p *Client) addActualAcknowledgerAsNoteToIncident(incident *pagerduty.Incident, actualAcknowledger string) error {
	noteContent := fmt.Sprintf("Incident was acknowledged on behalf of %s. time: %s", actualAcknowledger, time.Now().UTC().String())
	p.logger.LogDebug(
		"adding note to incident",
		"incidentID", incident.Id,
		"content", noteContent,
	)
	note := pagerduty.IncidentNote{
		Content: noteContent,
		User: pagerduty.APIObject{
			ID:      p.defaultUser.ID,
			Type:    TypeUserReference,
			Self:    p.defaultUser.Self,
			HTMLURL: p.defaultUser.HTMLURL,
			Summary: p.defaultUser.Summary,
		},
	}
	return p.pagerdutyClient.CreateIncidentNote(incident.Id, p.defaultUser.Email, note)
}

func acknowledgeIncident(incident *pagerduty.Incident, user *pagerduty.User) pagerduty.Incident {
	timeNowUTCString := time.Now().UTC().String()
	ackedIncident := *incident

	ackedIncident.Acknowledgements = append(ackedIncident.Acknowledgements, pagerduty.Acknowledgement{
		At: timeNowUTCString,
		Acknowledger: pagerduty.APIObject{
			Type:    TypeUserReference,
			ID:      user.ID,
			Summary: user.Summary,
			HTMLURL: user.HTMLURL,
			Self:    user.Self,
		},
	})
	ackedIncident.Assignments = append(ackedIncident.Assignments, pagerduty.Assignment{
		At: timeNowUTCString,
		Assignee: pagerduty.APIObject{
			Type:    TypeUserReference,
			ID:      user.ID,
			Summary: user.Summary,
			HTMLURL: user.HTMLURL,
			Self:    user.Self,
		},
	})
	ackedIncident.Status = StatusAcknowledged
	return ackedIncident
}

func isUserNotFound(err error) bool {
	return err.Error() == ErrUserNotFound.Error()
}
