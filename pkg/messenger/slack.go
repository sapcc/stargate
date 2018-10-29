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

package messenger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/nlopes/slack"
	"github.com/nlopes/slack/slackevents"
	"github.com/pkg/errors"
	"github.com/prometheus/common/model"

	"github.com/sapcc/stargate/pkg/alertmanager"
	"github.com/sapcc/stargate/pkg/api"
	"github.com/sapcc/stargate/pkg/config"
	"github.com/sapcc/stargate/pkg/util"
)

const (
	// Silence action type
	Silence = "silence"

	// Acknowledge action type
	Acknowledge = "acknowledge"

	// ClientUserName appears in the messenger message
	PostAsUserName = "stargate"

	// ReactionSilenced is applied to a message after it was successfully silenced
	ReactionSilenced = "silent-bell"
)

type slackClient struct {
	config config.Config

	// list of slack user ids that are authorized to interact with stargate messages
	authorizedUsers []string

	slackClient        *slack.Client
	alertmanagerClient alertmanager.Alertmanager
}

// NewSlackClient returns a new receiver
func NewSlackClient(config config.Config) Receiver {
	s := slack.New(config.SlackConfig.AccessToken)
	s.SetDebug(true)

	slackClient :=  &slackClient{
		config:             config,
		alertmanagerClient: alertmanager.New(config),
		slackClient:        s,
	}

	if err := slackClient.getAuthorizedSlackUserGroupMembers(); err != nil {
		log.Fatal(err)
	}

	return slackClient
}

// HandleMessage handles a messenger message
func (s *slackClient) HandleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.HandleError(405, "This endpoint only supports POST", w)
		return
	}

	log.Println("received slack message")

	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Body)

	slackMessageAction, err := slackevents.ParseActionEvent(
		buf.String(),
		slackevents.OptionVerifyToken(&slackevents.TokenComparator{VerificationToken: s.config.SlackConfig.GetValidationToken()}),
	)

	if err != nil {
		code := 500
		if isErrorInvalidToken(err) {
			code = 401
		}
		json.NewEncoder(w).Encode(
			api.Error{
				Code:    code,
				Message: err.Error(),
			},
		)
		log.Printf("Failed to read request body: %v", err)
		return
	}

	if slackMessageAction.Type == slackevents.URLVerification {
		var c *slackevents.ChallengeResponse
		err := json.Unmarshal([]byte(buf.String()), &c)
		if err != nil {
			json.NewEncoder(w).Encode(
				api.Error{
					Code:    500,
					Message: err.Error(),
				},
			)
		}
		w.Header().Set("Content-Type", "text")
		w.Write([]byte(c.Challenge))
	}

	for _, action := range slackMessageAction.Actions {
		// only react to buttons clicks
		if action.Name != "reaction" {
			continue
		}
		switch action.Value {
		case Silence:
			//TODO:

			s.addReactionToMessage(slackMessageAction.OriginalMessage, ReactionSilenced)
			continue

		default:
			log.Printf("not responding to action '%s'", action.Value)
		}
	}
}

func (s *slackClient) checkAction(messageAction slackevents.MessageAction) {
	for _, action := range messageAction.Actions {
		// only react to buttons clicks
		if action.Name != "reaction" {
			continue
		}
		switch action.Value {
		// create a silence
		case Silence:
			alert := s.alertFromSlackMessage(messageAction.OriginalMessage)

			//TODO: comment, duration
			err := s.alertmanagerClient.CreateSilence(
				alert,
				messageAction.User.Id,
				"default comment",
				10*time.Minute,
			)
			if err != nil {
				log.Printf("error creating silence: %v", err)
			}

			// Confirm the silence was successfully created by posting to the channel
			s.PostToChannel(messageAction.Channel.Id, fmt.Sprintf("Created silence for alert %s", alert.Name()))

		default:
			log.Printf("not responding to action '%s'", action.Value)
		}
	}
}

//TODO: which fields does the messenger messenger contain
func (s *slackClient) alertFromSlackMessage(message slack.Message) *model.Alert {
	return &model.Alert{
		Labels: model.LabelSet{
			model.AlertNameLabel: model.LabelValue(message.Text),
		},
	}
}

func isErrorInvalidToken(err error) bool {
	if err.Error() == "invalid verification token" {
		return true
	}
	return false
}

func (s *slackClient) PostToChannel(channel, message string) {
	s.slackClient.PostMessage(
		channel,
		message,
		slack.PostMessageParameters{
			Username: PostAsUserName,
		},
	)
}

func (s *slackClient) getAuthorizedSlackUserGroupMembers() error {
	userGroupIDs, err := s.userGroupNamesToIDs(s.config.SlackConfig.AuthorizedGroups)
	if err != nil {
		return err
	}

	var authorizedUsers []string
	for _, groupID := range userGroupIDs {
		members, err := s.slackClient.GetUserGroupMembers(groupID)
		if err != nil {
			log.Printf("error while getting members of group %s: %v", groupID, err)
			continue
		}
		authorizedUsers = append(authorizedUsers, members...)
	}

	if authorizedUsers == nil {
		return errors.New("not a single user is authorized to respond to slack messages. check config")
	}

	s.authorizedUsers = authorizedUsers
	return nil
}

// for convenience slack user groups are configured by name rather than ID.
// userGroupNamesToIDs finds the ID based on the name of a slack user group
func (s *slackClient) userGroupNamesToIDs(userGroupNames []string) ([]string, error) {
	userGroupIDs := make([]string, 0)

	userGroups, err := s.slackClient.GetUserGroups()
	if err != nil {
		return userGroupIDs, err
	}

	for _, group := range userGroups {
		if util.StringSliceContains(userGroupNames, group.Name) {
			userGroupIDs = append(userGroupIDs, group.ID)
		}
	}

	return userGroupIDs, nil
}

func (s *slackClient) isUserAuthorized(userName string) bool {
	return util.StringSliceContains(s.authorizedUsers, userName)
}

func (s *slackClient) addReactionToMessage(message slack.Message, reaction string) {
	msgRef := slack.NewRefToMessage(message.Channel, message.Timestamp)
	if err := s.slackClient.AddReaction(reaction, msgRef); err != nil {
		log.Printf("error adding reaction to message: %v", err)
	}

}
