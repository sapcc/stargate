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
	"log"
	"time"

	"github.com/nlopes/slack/slackevents"
	"github.com/sapcc/stargate/pkg/util"
)

func (s *slackClient) HandleSlackMessageActionEvent(payload string) {
	if payload == "" {
		log.Printf("empty paylod. request does not contain a slack message action event")
		return
	}

	slackMessageAction, err := slackevents.ParseActionEvent(
		payload,
		slackevents.OptionVerifyToken(&slackevents.TokenComparator{VerificationToken: s.config.SlackConfig.GetValidationToken()}),
	)
	if err != nil {
		if isErrorInvalidToken(err) {
			log.Printf("failed to verify slack message: %v", err)
			return
		}
		log.Printf("failed to unmarshal request body: %v", err)
		return
	}
	if !s.isUserAuthorized(slackMessageAction.User.Id) {
		log.Printf("user with ID '%s' is not authorized to respond to a message", slackMessageAction.User.Id)
		return
	}

	if err := s.checkAction(slackMessageAction); err != nil {
		log.Printf("failed to respond to slack message: %v", err)
	}
}

// check which action was selected (if any)
func (s *slackClient) checkAction(messageAction slackevents.MessageAction) error {
	for _, action := range messageAction.Actions {
		// only react to buttons clicks
		if action.Name != ActionName || action.Type != ActionType {
			log.Printf("ignoring action with name '%s', type '%s', value '%s'", action.Name, action.Type, action.Value)
			continue
		}

		switch action.Value {

		case reactionTypes.Acknowledge:
			if err := s.acknowledgeAlert(messageAction); err != nil {
				log.Printf("failed to acknowledge: %v", err)
			}

		case reactionTypes.SilenceUntilMonday:
			durationDays := util.TimeUntilNextMonday(time.Now().UTC())
			if err := s.silenceAlert(messageAction, util.DaysToHours(durationDays)); err != nil {
				log.Printf("error creating silence: %v", err)
			}

		case reactionTypes.Silence1Day:
			if err := s.silenceAlert(messageAction, util.DaysToHours(1)); err != nil {
				log.Printf("error creating silence: %v", err)
			}

		case reactionTypes.Silence1Month:
			if err := s.silenceAlert(messageAction, util.DaysToHours(31)); err != nil {
				log.Printf("error creating silence: %v", err)
			}

		default:
			log.Printf("not responding to action '%s'", action.Value)
		}
	}

	return nil
}
