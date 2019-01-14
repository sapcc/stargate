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

package stargate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sapcc/stargate/pkg/alertmanager"
	"github.com/sapcc/stargate/pkg/api"
	"github.com/sapcc/stargate/pkg/config"
	"github.com/sapcc/stargate/pkg/log"
	"github.com/sapcc/stargate/pkg/metrics"
	"github.com/sapcc/stargate/pkg/pagerduty"
	"github.com/sapcc/stargate/pkg/slack"
	"github.com/sapcc/stargate/pkg/store"
	"github.com/sapcc/stargate/pkg/util"
)

// Stargate ...
type Stargate struct {
	v1API              *api.API
	logger             log.Logger
	alertmanagerClient alertmanager.Alertmanager
	pagerdutyClient    *pagerduty.Client
	slack              *slack.Client
	opts               config.Options
	alertStore         *store.AlertStore

	Config config.Config
}

// New creates a new stargate
func New(opts config.Options) *Stargate {
	logger := log.NewLogger()

	cfg, err := config.NewConfig(opts, logger)
	if err != nil {
		logger.LogFatal("failed to load configuration", "err", err)
	}

	persister, err := store.NewFilePersister(opts.PersistenceFilePath, opts.GCInterval, logger)
	if err != nil {
		logger.LogError("failed to create persister. running in stateless mode", err)
	}

	sg := &Stargate{
		Config:             cfg,
		slack:              slack.NewClient(cfg, opts, logger),
		opts:               opts,
		alertmanagerClient: alertmanager.New(cfg, logger),
		pagerdutyClient:    pagerduty.NewClient(cfg, logger),
		alertStore:         store.NewAlertStore(opts.GCInterval, persister, logger),
		logger:             logger,
	}

	v1API := api.NewAPI(cfg, logger)

	// the v1 endpoint that accepts slack message action events
	v1API.AddRouteV1(http.MethodPost, "/slack/event", sg.HandleSlackMessageActionEvent)

	// the v1 endpoint that accepts slack commands
	v1API.AddRouteV1(http.MethodPost, "/slack/command", sg.HandleSlackCommand)

	v1API.AddRouteV1(http.MethodGet, "/alerts", sg.HandleListAlerts)

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

	// start API
	go func() {
		if err := s.v1API.Serve(); err != nil {
			s.logger.LogFatal("stargate API failed with", "err", err)
		}
	}()

	// start alert store
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.alertStore.Run(ctx)

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

func (s *Stargate) HandleListAlerts(w http.ResponseWriter, r *http.Request) {
	alertList, err := s.alertmanagerClient.ListAlerts(map[string]string{})
	if err != nil {
		s.logger.LogError("error listing alerts", err)
	}

	for idx, alert := range alertList {
		a, err := s.alertStore.GetFromFingerPrintString(alert.Fingerprint)
		if err != nil {
			alertList[idx].Annotations = util.MergeAnnotations(a, alert)
		}
	}

	json.NewEncoder(w).Encode(alertList)
}

// HandleSlackMessageActionEvent handles slack message action events
func (s *Stargate) HandleSlackMessageActionEvent(w http.ResponseWriter, r *http.Request) {
	s.logger.LogDebug("received slack message action event")
	w.WriteHeader(http.StatusNoContent)
	if err := r.ParseForm(); err != nil {
		s.logger.LogError("failed to parse request", err)
		return
	}
	var payloadString string
	for k, v := range r.Form {
		if k == "payload" && len(v) == 1 {
			payloadString = v[0]
			break
		}
	}

	go func() {
		slackMessageAction, err := s.slack.MessageActionFromPayload(payloadString)
		if err != nil {
			s.logger.LogError("failed to parse slack message", err)
		}

		var userName string
		userName, err = s.slack.GetUserNameByID(slackMessageAction.User.Id)
		if err != nil {
			s.logger.LogError("user not found by id", err, "userID", slackMessageAction.User.Id, "userName", userName)
		}

		// check whether user is authorized
		if !s.slack.IsUserAuthorized(slackMessageAction.User.Id) {
			s.logger.LogInfo("user is not authorized to respond to a message",
				"userID", slackMessageAction.User.Id,
				"userName", userName,
			)
			return
		}

		slackAlert, err := s.slack.AlertFromSlackMessage(slackMessageAction)
		if err != nil {
			s.logger.LogError("failed to parse alert from slack message", err)
		}

		actionList, err := s.slack.ActionFromSlackMessage(slackMessageAction)
		if err != nil {
			s.logger.LogError("failed to parse actions from slack message", err)
		}

		userEmail, err := s.slack.GetUserEmailByID(slackMessageAction.User.Id)
		if err != nil {
			s.logger.LogError("failed to get email of user", err, "userID", slackMessageAction.User.Id, "userName", userName)
		}

		for _, action := range actionList {
			switch action {

			// Acknowledge an alert
			case slack.Reaction.Acknowledge:
				alertList, err := s.alertmanagerClient.AcknowledgeAlert(slackAlert, userName)
				if err != nil {
					s.logger.LogError("failed to acknowledge alerts", err)
				}
				for _, alert := range alertList {
					if err := s.alertStore.SetFromExtendedAlert(alert); err != nil {
						s.logger.LogError("failed to set alert in store", err)
						metrics.FailedOperationsTotal.WithLabelValues("acknowledge").Inc()
					}

					if err := s.pagerdutyClient.AcknowledgeIncident(alert, userEmail); err != nil {
						s.logger.LogError("failed to acknowledge incident", err)
						metrics.FailedOperationsTotal.WithLabelValues("acknowledge").Inc()
					}
				}

				s.slack.PostMessage(
					slackMessageAction.Channel.Id,
					fmt.Sprintf("Acknowledged by <@%s>", slackMessageAction.User.Id),
					slackMessageAction.OriginalMessage.Timestamp,
				)
				s.slack.AddReactionToMessage(slackMessageAction.Channel.Id, slackMessageAction.OriginalMessage.Timestamp, slack.AcknowledgeReactionEmoji)

				metrics.SuccessfulOperationsTotal.WithLabelValues("acknowledge").Inc()

			// Create a silence until next monday
			case slack.Reaction.SilenceUntilMonday:
				durationDays := util.TimeUntilNextMonday(time.Now().UTC())
				silenceID, err := s.alertmanagerClient.CreateSilence(slackAlert, userName, slack.SilenceDefaultComment, util.DaysToHours(durationDays))
				if err != nil {
					s.logger.LogError("error creating silence", err)
					metrics.FailedOperationsTotal.WithLabelValues("silence").Inc()
				}

				s.slack.PostMessage(
					slackMessageAction.Channel.Id,
					fmt.Sprintf("<@%s> silenced alert %s for %s. <%s|See Silence>", slackMessageAction.User.Id, slackAlert.Name(), util.HumanizedDurationString(durationDays), s.alertmanagerClient.LinkToSilence(silenceID)),
					slackMessageAction.OriginalMessage.Timestamp,
				)
				s.slack.AddReactionToMessage(slackMessageAction.Channel.Id, slackMessageAction.OriginalMessage.Timestamp, slack.SilenceSuccessReactionEmoji)

				metrics.SuccessfulOperationsTotal.WithLabelValues("silence").Inc()

			// Create a silence for 1 day
			case slack.Reaction.Silence1Day:
				durationHours := util.DaysToHours(1)
				silenceID, err := s.alertmanagerClient.CreateSilence(slackAlert, userName, slack.SilenceDefaultComment, durationHours)
				if err != nil {
					s.logger.LogError("error creating silence", err)
					metrics.FailedOperationsTotal.WithLabelValues("silence").Inc()
				}

				s.slack.PostMessage(
					slackMessageAction.Channel.Id,
					fmt.Sprintf("<@%s> silenced alert %s for %s. <%s|See Silence>", slackMessageAction.User.Id, slackAlert.Name(), util.HumanizedDurationString(durationHours), s.alertmanagerClient.LinkToSilence(silenceID)),
					slackMessageAction.OriginalMessage.Timestamp,
				)
				s.slack.AddReactionToMessage(slackMessageAction.Channel.Id, slackMessageAction.OriginalMessage.Timestamp, slack.SilenceSuccessReactionEmoji)

				metrics.SuccessfulOperationsTotal.WithLabelValues("silence").Inc()

			// Create a silence for 1 month
			case slack.Reaction.Silence1Month:
				durationHours := util.DaysToHours(31)
				silenceID, err := s.alertmanagerClient.CreateSilence(slackAlert, userName, slack.SilenceDefaultComment, durationHours)
				if err != nil {
					s.logger.LogError("error creating silence", err)
					metrics.FailedOperationsTotal.WithLabelValues("silence").Inc()
				}

				s.slack.PostMessage(
					slackMessageAction.Channel.Id,
					fmt.Sprintf("<@%s> silenced alert %s for %s. <%s|See Silence>", slackMessageAction.User.Id, slackAlert.Name(), util.HumanizedDurationString(durationHours), s.alertmanagerClient.LinkToSilence(silenceID)),
					slackMessageAction.OriginalMessage.Timestamp,
				)
				s.slack.AddReactionToMessage(slackMessageAction.Channel.Id, slackMessageAction.OriginalMessage.Timestamp, slack.SilenceSuccessReactionEmoji)

				metrics.SuccessfulOperationsTotal.WithLabelValues("silence").Inc()

			default:
				s.logger.LogDebug("not responding to action", "actionValue", action)
			}
		}
	}()
}
