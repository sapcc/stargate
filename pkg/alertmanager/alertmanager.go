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

package alertmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/alertmanager/client"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/client_golang/api"
	"github.com/sapcc/stargate/pkg/config"
	"github.com/sapcc/stargate/pkg/log"
)

const (
	// AcknowledgedByLabel ...
	AcknowledgedByLabel = "acknowledgedBy"

	// RegionLabel ...
	RegionLabel = "region"

	// SeverityLabel ...
	SeverityLabel = "severity"
)

// Client ...
type Client struct {
	Config config.Config

	logger           log.Logger
	silenceAPIClient client.SilenceAPI
	alertAPIClient   client.AlertAPI
}

// New creates a new Alertmanager client.
func New(config config.Config, logger log.Logger) *Client {
	logger = log.NewLoggerWith(logger, "component", "alertmanager")

	apiClient, err := api.NewClient(api.Config{Address: config.AlertManager.URL})
	if err != nil {
		logger.LogFatal("failed to create alertmanager api client", "alertmanagerURL", config.AlertManager.URL, "err", err)
	}

	return &Client{
		Config:           config,
		logger:           logger,
		silenceAPIClient: client.NewSilenceAPI(apiClient),
		alertAPIClient:   client.NewAlertAPI(apiClient),
	}
}

// CreateSilence creates a silence.
func (a *Client) CreateSilence(alert *client.ExtendedAlert, silenceAuthor, silenceComment string, silenceDuration time.Duration) (string, error) {
	if alert == nil {
		return "", errors.New("alert must not be nil")
	}
	if silenceDuration == 0 {
		return "", errors.New("duration must be greater than 0")
	}
	if silenceAuthor == "" {
		return "", errors.New("author must not be empty")
	}

	a.logger.LogInfo("creating silence",
		"alertLabels", alert.Labels,
		"silenceDuration", silenceDuration,
		"silenceAuthor", silenceAuthor,
	)

	now := time.Now().UTC()
	silenceMatchers := matchersFromAlert(alert)

	silenceID, isExists, err := a.isSilenceExists(silenceMatchers)
	if err != nil {
		return "", err
	}
	if isExists {
		a.logger.LogInfo("silence already exists", "silenceMatchers", silenceMatchers)
		return silenceID, err
	}

	silence := types.Silence{
		Matchers:  silenceMatchers,
		StartsAt:  now,
		EndsAt:    now.Add(silenceDuration),
		CreatedBy: silenceAuthor,
		Comment:   silenceComment,
	}

	silenceID, err = a.silenceAPIClient.Set(context.TODO(), silence)
	if err != nil {
		return "", err
	}
	a.logger.LogInfo("created silence", "silenceID", silenceID)

	return silenceID, nil
}

// LinkToSilence creates a link to a silence.
func (a *Client) LinkToSilence(silenceID string) string {
	return fmt.Sprintf("%s/#/silences/%s", a.Config.AlertManager.URL, silenceID)
}

// ListAlerts returns a list of alerts or an error.
func (a *Client) ListAlerts(f *Filter) ([]*client.ExtendedAlert, error) {
	return a.alertAPIClient.List(
		context.TODO(), f.toString(), f.Receiver, f.IsSilenced, f.IsInhibited, f.IsActive, f.IsUnprocessed,
	)
}

func (a *Client) isSilenceExists(matchers types.Matchers) (string, bool, error) {
	matchersNoAuthor := matchersWithoutAuthor(matchers)
	silences, err := a.silenceAPIClient.List(context.TODO(), matchersNoAuthor.String())
	if err != nil {
		return "", false, err
	}
	for _, s := range silences {
		if matchersWithoutAuthor(s.Matchers).Equal(matchersNoAuthor) {
			return s.ID, true, nil
		}
	}
	return "", false, nil
}

func matchersWithoutAuthor(matchers types.Matchers) types.Matchers {
	matcherWithoutAuthor := make([]*types.Matcher, 0)
	for _, m := range matchers {
		if m.Name != "createdBy" {
			matcherWithoutAuthor = append(matcherWithoutAuthor, m)
		}
	}
	return matcherWithoutAuthor
}

func matchersFromAlert(alert *client.ExtendedAlert) types.Matchers {
	matchers := make([]*types.Matcher, 0)
	for labelKey, labelValue := range alert.Labels {
		m := &types.Matcher{
			Name:  string(labelKey),
			Value: string(labelValue),
		}
		m.IsRegex = true

		matchers = append(matchers, m)
	}

	return matchers
}
