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

package alertmanager

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/alertmanager/client"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/common/model"
	"github.com/sapcc/stargate/pkg/config"
)

// LabelNameAcknowledgedBy ...
const LabelNameAcknowledgedBy = "acknowledgedBy"

type alertmanagerClient struct {
	Alertmanager
	Config config.Config

	silenceAPIClient client.SilenceAPI
	alertAPIClient   client.AlertAPI
}

// New creates a new Alertmanager
func New(config config.Config) Alertmanager {
	apiClient, err := api.NewClient(api.Config{Address: config.AlertManager.URL})
	if err != nil {
		log.Fatalf("failed to create alertmanager api client using url '%s': %v", config.AlertManager.URL, err)
	}

	return &alertmanagerClient{
		Config:           config,
		silenceAPIClient: client.NewSilenceAPI(apiClient),
		alertAPIClient:   client.NewAlertAPI(apiClient),
	}
}

func (a *alertmanagerClient) CreateSilence(alert *model.Alert, silenceAuthor, silenceComment string, silenceDuration time.Duration) (string, error) {
	if alert == nil {
		return "", errors.New("alert must not be nil")
	}
	if silenceDuration == 0 {
		return "", errors.New("duration must be greater than 0")
	}
	if silenceAuthor == "" {
		return "", errors.New("author must not be empty")
	}

	log.Printf("creating silence for alert: %v, duration: %v, author: %s", alert.Labels, silenceDuration, silenceAuthor)

	now := time.Now().UTC()
	silenceMatchers := matchersFromAlert(alert)

	silenceID, isExists, err := a.isSilenceExists(silenceMatchers)
	if err != nil {
		return "", err
	}
	if isExists {
		log.Printf("silence with matchers %v already exists. not creating again", silenceMatchers)
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
	log.Printf("created silence with ID '%s'", silenceID)

	return silenceID, nil
}

func (a *alertmanagerClient) isSilenceExists(matchers types.Matchers) (string, bool, error) {
	mathersNoAuthor := matchersWithoutAuthor(matchers)
	silences, err := a.silenceAPIClient.List(context.TODO(), mathersNoAuthor.String())
	if err != nil {
		return "", false, err
	}
	for _, s := range silences {
		if matchersWithoutAuthor(s.Matchers).Equal(mathersNoAuthor) {
			return s.ID, true, nil
		}
	}
	return "", false, nil
}

func (a *alertmanagerClient) LinkToSilence(silenceID string) string {
	return fmt.Sprintf("%s/#/silences/%s", a.Config.AlertManager.URL, silenceID)
}

func (a *alertmanagerClient) AcknowledgeAlert(alert *model.Alert, acknowledgedBy string) error {
	alertList, err := a.findAlerts(alert)
	if err != nil {
		return err
	}

	for idx, a := range alertList {
		ack, ok := a.Labels[LabelNameAcknowledgedBy]
		if ok && !strings.Contains(string(ack), acknowledgedBy) {
			acknowledgedBy = fmt.Sprintf("%s, %s", ack, acknowledgedBy)
		}
		alertList[idx].Labels[LabelNameAcknowledgedBy] = client.LabelValue(acknowledgedBy)
	}

	return a.alertAPIClient.Push(
		context.TODO(),
		alertList...,
	)
}

func (a *alertmanagerClient) ListAlerts(filter map[string]string) ([]*client.ExtendedAlert, error) {
	var filterList []string
	for k, v := range filter {
		filterList = append(filterList, fmt.Sprintf(`%s="%s"`, k, v))
	}
	return a.alertAPIClient.List(
		context.TODO(), strings.Join(filterList, ","), "", true, true, true, false,
	)
}

func (a *alertmanagerClient) findAlerts(alert *model.Alert) ([]client.Alert, error) {
	var filterList []string
	for labelName, labelValue := range alert.Labels {
		filterList = append(filterList, fmt.Sprintf(`%s="%s"`, labelName, labelValue))
	}

	extendedAlertList, err := a.alertAPIClient.List(
		context.TODO(), strings.Join(filterList, ","), "", true, true, true, false,
	)
	if err != nil {
		return nil, err
	}

	if len(extendedAlertList) == 0 {
		return nil, fmt.Errorf("no alert(s) with name '%v' found", alert.Labels[model.AlertNameLabel])
	}

	var alertList = make([]client.Alert, 0)
	for _, al := range extendedAlertList {
		alertList = append(alertList, al.Alert)
	}

	return alertList, nil
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

func matchersFromAlert(alert *model.Alert) types.Matchers {
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
