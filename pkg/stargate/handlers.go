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

			// Acknowledge an alert.
			case slack.Reaction.Acknowledge:
				// At least post the message to slack.
				s.slack.PostMessage(
					slackMessageAction.Channel.Id,
					fmt.Sprintf("Acknowledged by <@%s>", slackMessageAction.User.Id),
					slackMessageAction.OriginalMessage.Timestamp,
				)
				s.slack.AddReactionToMessage(slackMessageAction.Channel.Id, slackMessageAction.OriginalMessage.Timestamp, slack.AcknowledgeReactionEmoji)

				// List all alerts that match the slack alert.
				filter := alertmanager.NewDefaultFilter()
				filter.WithAlertLabelsFilter(slackAlert.Labels)
				alertList, err := s.alertmanagerClient.ListAlerts(filter)
				if err != nil {
					s.logger.LogError("failed to get list alerts from alertmanager", err)
					return
				}

				// Acknowledge the alerts matching the labels found in the slack message.
				err = s.alertStore.AcknowledgeAndSetMultiple(alertList, userName)
				if err != nil {
					s.logger.LogError("failed to acknowledge alert", err, "component", "alertmanager", "labels", alert.ClientLabelSetToString(slackAlert.Labels))
					metrics.FailedOperationsTotal.WithLabelValues("acknowledge").Inc()
				} else {
					// Don't return here on failure. We might be able to acknowledge in Pagerduty.
					s.logger.LogInfo("acknowledged alert", "component", "alertmanager", "labels", alert.ClientLabelSetToString(slackAlert.Labels))
				}

				if err := s.pagerdutyClient.AcknowledgeIncident(slackAlert, userEmail); err != nil {
					s.logger.LogError("failed to acknowledge incident", err, "component", "pagerduty")
					metrics.FailedOperationsTotal.WithLabelValues("acknowledge").Inc()
					return
				}
				s.logger.LogInfo("acknowledged alert", "component", "pagerduty", "labels", alert.ClientLabelSetToString(slackAlert.Labels))
				metrics.SuccessfulOperationsTotal.WithLabelValues("acknowledge").Inc()

				// Create a silence until next monday
			case slack.Reaction.SilenceUntilMonday:
				durationDays := util.TimeUntilNextMonday(time.Now().UTC())
				silenceID, err := s.alertmanagerClient.CreateSilence(slackAlert, userName, slack.SilenceDefaultComment, util.DaysToHours(durationDays))
				if err != nil {
					s.logger.LogError("error creating silence", err, "component", "alertmanager")
					metrics.FailedOperationsTotal.WithLabelValues("silence").Inc()
					return
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
					s.logger.LogError("error creating silence", err, "component", "alertmanager")
					metrics.FailedOperationsTotal.WithLabelValues("silence").Inc()
					return
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
					s.logger.LogError("error creating silence", err, "component", "alertmanager")
					metrics.FailedOperationsTotal.WithLabelValues("silence").Inc()
					return
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
	w.WriteHeader(http.StatusOK)
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

	err = json.NewEncoder(w).Encode(alertList)
	if err != nil {
		s.logger.LogError("error encoding alertList", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(api.Error{Code: http.StatusInternalServerError, Message: "error encoding list of alerts"})
		return
	}
	s.logger.LogDebug("responding to request", "handler", "listAlerts")
}

// HandleListSilences handles silence listing.
func (s *Stargate) HandleListSilences(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	f := alertmanager.NewFilterFromRequest(r)
	silenceList, err := s.alertmanagerClient.ListSilences(f)
	if err != nil {
		s.logger.LogError("error getting list of silences", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(api.Error{Code: http.StatusInternalServerError, Message: "error getting list of silences"})
		return
	}

	err = json.NewEncoder(w).Encode(silenceList)
	if err != nil {
		s.logger.LogError("error encoding silenceList", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(api.Error{Code: http.StatusInternalServerError, Message: "error encoding list of silences"})
	}
	s.logger.LogDebug("responding to request", "handler", "listAlerts")
}

// HandleGetStatus handles the status.
func (s *Stargate) HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	if err != nil {
		s.logger.LogError("error encoding status", err)
		json.NewEncoder(w).Encode(api.Error{Code: http.StatusInternalServerError, Message: "error encoding status"})
	}
}
