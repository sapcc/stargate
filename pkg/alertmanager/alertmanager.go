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
	"log"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/alertmanager/client"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/common/model"
	"github.com/sapcc/stargate/pkg/config"
)

type alertmanagerClient struct {
	Alertmanager

	silenceAPIClient client.SilenceAPI
}

// New creates a new Alertmanager
func New(config config.Config) Alertmanager {
	apiClient, err := api.NewClient(api.Config{Address: config.AlertManager.URL})
	if err != nil {
		log.Fatalf("failed to create alertmanager api client using url '%s': %v", config.AlertManager.URL, err)
	}

	silenceAPI := client.NewSilenceAPI(apiClient)

	return &alertmanagerClient{
		silenceAPIClient: silenceAPI,
	}
}

func (a *alertmanagerClient) CreateSilence(alert *model.Alert, author, comment string, duration time.Duration) error {
	if alert == nil {
		return errors.New("alert must not be nil")
	}
	if duration == 0 {
		return errors.New("duration must be greater than 0")
	}
	if author == "" {
		return errors.New("author must no be empty")
	}

	now := time.Now().UTC()

	silence := types.Silence{
		Matchers:  matchersFromAlert(alert),
		StartsAt:  now,
		EndsAt:    now.Add(duration),
		CreatedBy: author,
		Comment:   comment,
	}

	silenceID, err := a.silenceAPIClient.Set(context.TODO(), silence)
	if err != nil {
		return err
	}
	log.Printf("created silence with ID '%s'", silenceID)

	return nil
}

func matchersFromAlert(alert *model.Alert) types.Matchers {
	matchers := make([]*types.Matcher, 0)
	for labelKey, labelValue := range alert.Labels {
		m := &types.Matcher{
			Name: string(labelKey),
			Value: string(labelValue),
		}
		m.IsRegex = true

		matchers = append(matchers,m)
	}

	return matchers
}

