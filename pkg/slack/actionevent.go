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

package slack

import (
	"time"

	"github.com/nlopes/slack/slackevents"
	"github.com/sapcc/stargate/pkg/util"
)

func (s *slackClient) HandleSlackMessageActionEvent(payload string) {
	if payload == "" {
		s.logger.LogDebug("empty paylod. request does not contain a slack message action event")
		return
	}

	slackMessageAction, err := slackevents.ParseActionEvent(
		payload,
		slackevents.OptionVerifyToken(&slackevents.TokenComparator{VerificationToken: s.config.Slack.GetValidationToken()}),
	)
	if err != nil {
		if isErrorInvalidToken(err) {
			s.logger.LogError("failed to verify slack message", err)
			return
		}
		s.logger.LogError("failed to unmarshal request body", err)
		return
	}

	var userName string
	userName, err = s.slackUserIDToName(slackMessageAction.User.Id)
	if err != nil {
		s.logger.LogError("user not found by id", err, "userID", slackMessageAction.User.Id)
	}

	if !s.isUserAuthorized(slackMessageAction.User.Id) {
		s.logger.LogInfo("user is not authorized to respond to a message",
			"userID", slackMessageAction.User.Id,
			"userName", userName,
		)
		return
	}

	if err := s.checkAction(slackMessageAction); err != nil {
		s.logger.LogError("failed to respond to slack message", err)
	}
}

// check which action was selected (if any)
func (s *slackClient) checkAction(messageAction slackevents.MessageAction) error {
	for _, action := range messageAction.Actions {
		// only react to buttons clicks
		if action.Name != ActionName || action.Type != ActionType {
			s.logger.LogDebug("ignoring action",
				"actionName", action.Name,
				"actionType", action.Type,
				"actionValue", action.Value,
			)
			continue
		}

		switch action.Value {

		case Reaction.Acknowledge:
			if err := s.acknowledgeAlert(messageAction); err != nil {
				s.logger.LogError("failed to acknowledge alert", err)
			}

		case Reaction.SilenceUntilMonday:
			durationDays := util.TimeUntilNextMonday(time.Now().UTC())
			if err := s.silenceAlert(messageAction, util.DaysToHours(durationDays)); err != nil {
				s.logger.LogError("error creating silence", err)
			}

		case Reaction.Silence1Day:
			if err := s.silenceAlert(messageAction, util.DaysToHours(1)); err != nil {
				s.logger.LogError("error creating silence", err)
			}

		case Reaction.Silence1Month:
			if err := s.silenceAlert(messageAction, util.DaysToHours(31)); err != nil {
				s.logger.LogError("error creating silence", err)
			}

		default:
			s.logger.LogDebug("not responding to action", "actionValue", action.Value)
		}
	}

	return nil
}
