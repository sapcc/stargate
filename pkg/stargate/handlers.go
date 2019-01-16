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
	"fmt"
	"net/http"
	"time"

	"github.com/sapcc/stargate/pkg/alert"
	"github.com/sapcc/stargate/pkg/alertmanager"
	"github.com/sapcc/stargate/pkg/api"
	"github.com/sapcc/stargate/pkg/metrics"
	"github.com/sapcc/stargate/pkg/slack"
	"github.com/sapcc/stargate/pkg/store"
	"github.com/sapcc/stargate/pkg/util"
)

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

		slackAlert, err := s.slack.AlertFromSlackMessage(slackMessageAction.OriginalMessage)
		if err != nil {
			s.logger.LogError("failed to parse alert from slack message", err)
		}

		alertname, err := alert.GetAlertnameFromExtendedAlert(slackAlert)
		if err != nil {
			s.logger.LogError("failed to get alertname", err)
			return
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
				err := s.alertStore.AcknowledgeAlert(slackAlert, userName)
				if err != nil {
					s.logger.LogError("failed to acknowledge alert", err, "labels", alert.ClientLabelSetToString(slackAlert.Labels))
					metrics.FailedOperationsTotal.WithLabelValues("acknowledge").Inc()
				}

				if err := s.pagerdutyClient.AcknowledgeIncident(slackAlert, userEmail); err != nil {
					s.logger.LogError("failed to acknowledge incident", err)
					metrics.FailedOperationsTotal.WithLabelValues("acknowledge").Inc()
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
					fmt.Sprintf("<@%s> silenced alert %s for %v. <%s|See Silence>", slackMessageAction.User.Id, alertname, durationDays, s.alertmanagerClient.LinkToSilence(silenceID)),
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
					fmt.Sprintf("<@%s> silenced alert %s for %s. <%s|See Silence>", slackMessageAction.User.Id, alertname, util.HumanizedDurationString(durationHours), s.alertmanagerClient.LinkToSilence(silenceID)),
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
					fmt.Sprintf("<@%s> silenced alert %s for %s. <%s|See Silence>", slackMessageAction.User.Id, alertname, util.HumanizedDurationString(durationHours), s.alertmanagerClient.LinkToSilence(silenceID)),
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

// HandleListAlerts handles alert listing.
func (s *Stargate) HandleListAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
	userName, password, authOK := r.BasicAuth()
	if !authOK {
		json.NewEncoder(w).Encode(api.Error{Code: http.StatusUnauthorized, Message: "Unauthorized"})
		s.logger.LogInfo("unauthorized request", "handler", "listAlerts")
		return
	}

	if userName != s.Config.Slack.UserName || password != s.Config.Slack.GetValidationToken() {
		json.NewEncoder(w).Encode(api.Error{Code: http.StatusUnauthorized, Message: "Unauthorized"})
		s.logger.LogInfo("unauthorized request", "handler", "listAlerts")
		return
	}

	// get a fresh list of alerts from the alertmanager
	filter := alertmanager.NewFilterFromRequest(r)
	alertList, err := s.alertmanagerClient.ListAlerts(filter)
	if err != nil {
		s.logger.LogError("error listing alerts", err)
	}

	// if an alert is found in the internal alert store by its fingerprint,
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

	s.logger.LogDebug("responding to request", "handler", "listAlerts")
	json.NewEncoder(w).Encode(alertList)
}
