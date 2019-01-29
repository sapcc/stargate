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
	"sync"
	"time"

	"github.com/sapcc/stargate/pkg/alertmanager"
	"github.com/sapcc/stargate/pkg/api"
	"github.com/sapcc/stargate/pkg/config"
	"github.com/sapcc/stargate/pkg/log"
	"github.com/sapcc/stargate/pkg/pagerduty"
	"github.com/sapcc/stargate/pkg/slack"
	"github.com/sapcc/stargate/pkg/store"
)

// Stargate ...
type Stargate struct {
	v1API              *api.API
	logger             log.Logger
	alertmanagerClient *alertmanager.Client
	pagerdutyClient    *pagerduty.Client
	slack              *slack.Client
	opts               config.Options
	alertStore         *store.AlertStore

	Config config.Config
}

// New creates a new stargate
func New(opts config.Options, logger log.Logger) *Stargate {
	cfg, err := config.NewConfig(opts, logger)
	if err != nil {
		logger.LogFatal("failed to load configuration", "err", err)
	}

	persister, err := store.NewFilePersister(opts.PersistenceFilePath, logger)
	if err != nil {
		logger.LogFatal("failed to create persister", "err", err)
	}

	sg := &Stargate{
		Config:             cfg,
		slack:              slack.NewClient(cfg, opts, logger),
		opts:               opts,
		alertmanagerClient: alertmanager.New(cfg, logger),
		pagerdutyClient:    pagerduty.NewClient(cfg, logger),
		alertStore:         store.NewAlertStore(cfg, opts.RecheckInterval, persister, logger),
		logger:             logger,
	}

	v1API := api.NewAPI(cfg, logger)

	// the v1 endpoint that accepts slack message action events
	v1API.AddRouteV1(http.MethodPost, "/slack/event", sg.HandleSlackMessageActionEvent)

	// the v1 endpoint that accepts slack commands
	v1API.AddRouteV1(http.MethodPost, "/slack/command", sg.HandleSlackCommand)

	// the v1 endpoint that shows the status
	v1API.AddRouteV1(http.MethodGet, "/status", sg.HandleGetStatus)

	// the v1 endpoint that lists the alerts
	v1API.AddRouteV1WithBasicAuth(http.MethodGet, "/alerts", sg.HandleListAlerts)

	// the v1 endpoint that list silences
	v1API.AddRouteV1WithBasicAuth(http.MethodGet, "/silences", sg.HandleListSilences)

	// the v1 endpoint that gets a silence by id
	v1API.AddRouteV1WithBasicAuth(http.MethodGet, "/silence/{silenceID}", sg.HandleGetSilenceByID)

	sg.v1API = v1API
	return sg
}

// HandleSlackCommand handles slack commands
func (s *Stargate) HandleSlackCommand(w http.ResponseWriter, r *http.Request) {
	s.logger.LogDebug("received slack command")
	w.WriteHeader(http.StatusNoContent)
	r.ParseForm()

	go s.slack.HandleSlackCommand(r)
}

// Run starts the stargate
func (s *Stargate) Run(wg *sync.WaitGroup, stopCh <-chan struct{}) {
	defer wg.Done()
	wg.Add(2)

	ticker := time.NewTicker(s.Config.Slack.RecheckInterval)

	if !s.Config.Slack.IsDisableRTM {
		s.slack.RunRTM()
	}

	// start alert store
	go s.alertStore.Run(wg, stopCh)

	// start API
	go func() {
		if err := s.v1API.Serve(); err != nil {
			s.logger.LogFatal("stargate API failed with", "err", err)
		}
	}()

	// check whether members of authorized slack user groups have changed
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := s.slack.GetAuthorizedSlackUserGroupMembers(); err != nil {
					s.logger.LogError("error getting authorized slack user groups", err)
				}
			case <-stopCh:
				ticker.Stop()
				return
			}
		}
	}()
	<-stopCh
}
