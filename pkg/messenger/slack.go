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
	"log"
	"net/http"

	"github.com/nlopes/slack"
	"github.com/nlopes/slack/slackevents"
	"github.com/pkg/errors"
	"github.com/prometheus/common/model"

	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/sapcc/stargate/pkg/alertmanager"
	"github.com/sapcc/stargate/pkg/api"
	"github.com/sapcc/stargate/pkg/config"
	"github.com/sapcc/stargate/pkg/util"
)

const (
	// Silence8h action type
	Silence8h = "silence8h"

	// ActionName the name of the action the stargate is responding to
	ActionName = "reaction"

	// ActionType the type of the action the stargate is responding to
	ActionType = "button"

	// Acknowledge action type
	Acknowledge = "acknowledge"

	// PostAsUserName appears in the messenger message
	PostAsUserName = "stargate"

	// SilenceSuccessReactionEmoji is applied to a message after it was successfully silenced
	SilenceSuccessReactionEmoji = "silent-bell"

	// SilenceDefaultAuthor is the default author used for a silence
	SilenceDefaultAuthor = "stargate"

	// SilenceDefaultComment is the default comment used for a silence
	SilenceDefaultComment = "silenced by the stargate"

	// SeverityRegionRegex ...
	SeverityRegionRegex = `\*\[(?P<severity>.+)\]\* \*\[(?P<region>.+)\]\*.*\|(?P<alertname>.+)\>\* \- .+`
)

type slackClient struct {
	config config.Config

	// list of slack user ids that are authorized to interact with stargate messages
	authorizedUserIDs []string

	slackClient        *slack.Client
	alertmanagerClient alertmanager.Alertmanager
}

// NewSlackClient returns a new receiver
func NewSlackClient(config config.Config, isDebug bool) Receiver {
	s := slack.New(config.SlackConfig.AccessToken)
	s.SetDebug(isDebug)

	slackClient := &slackClient{
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
		api.RespondWithError(405, "This endpoint only supports POST", w)
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
			api.RespondWithUnauthorized(w)
			return
		}
		api.RespondWithError(code, err.Error(), w)
		log.Printf("failed to read request body: %v", err)
		return
	}

	if slackMessageAction.Type == slackevents.URLVerification {
		var c *slackevents.ChallengeResponse
		err := json.Unmarshal([]byte(buf.String()), &c)
		if err != nil {
			api.RespondWithError(500, err.Error(), w)
			log.Printf("failed to unmarshal request body: %v", err)
		}
		w.Header().Set("Content-Type", "text")
		w.Write([]byte(c.Challenge))
	}

	if !s.isUserAuthorized(slackMessageAction.User.Id) {
		api.RespondWithOK(w)
		log.Printf("user with ID '%s' is not authorized to respond to a message", slackMessageAction.User.Id)
		return
	}

	if err := s.checkAction(slackMessageAction); err != nil {
		api.RespondWithError(500, err.Error(), w)
		log.Printf("failed to respond to slack message: %v", err)
	}
}

func (s *slackClient) checkAction(messageAction slackevents.MessageAction) error {
	for _, action := range messageAction.Actions {
		// only react to buttons clicks
		if action.Name != ActionName || action.Type != ActionType {
			log.Printf("ignoring action with name '%s', type '%s', value '%s'", action.Name, action.Type, action.Value)
			continue
		}

		switch action.Value {
		// create a silence for 8h
		case Silence8h:
			alert, err := s.alertFromSlackMessage(messageAction.OriginalMessage)
			if err != nil {
				return errors.Wrapf(err, "failed to construct alert from slack message")
			}

			userName, err := s.slackUserIDToName(messageAction.User.Id)
			if err != nil {
				log.Printf("error finding slack user by id: %v", err)
				userName = SilenceDefaultAuthor
			}

			if err := s.alertmanagerClient.CreateSilence(
				alert,
				userName,
				SilenceDefaultComment,
				8*time.Hour,
			); err != nil {
				return errors.Wrapf(err, "error creating silence")
			}

			// Confirm the silence was successfully created by posting to the channel
			s.addReactionToMessage(messageAction.OriginalMessage, SilenceSuccessReactionEmoji)

			// Confirm the silence was successfully created by posting to the channel
			//s.postToChannel(messageAction.Channel.Id, fmt.Sprintf("Created silence for alert %s", alert.Name()))

		default:
			log.Printf("not responding to action '%s'", action.Value)
		}
	}

	return nil
}

func (s *slackClient) alertFromSlackMessage(message slack.Message) (*model.Alert, error) {
	labels, err := parseAlertFromSlackMessageText(message.Text)
	if err != nil {
		return &model.Alert{}, err
	}
	modelLabelset := model.LabelSet{}

	for k, v := range labels {
		modelLabelset[model.LabelName(k)] = model.LabelValue(v)
	}

	return &model.Alert{
		Labels: modelLabelset,
	}, nil
}

func isErrorInvalidToken(err error) bool {
	if err.Error() == "invalid verification token" {
		return true
	}
	return false
}

func (s *slackClient) getAuthorizedSlackUserGroupMembers() error {
	userGroupIDs, err := s.userGroupNamesToIDs(s.config.SlackConfig.AuthorizedGroups)
	if err != nil {
		return err
	}

	var authorizedUserIDs []string
	for _, groupID := range userGroupIDs {
		members, err := s.slackClient.GetUserGroupMembers(groupID)
		if err != nil {
			log.Printf("error while getting members of group %s: %v", groupID, err)
			continue
		}
		authorizedUserIDs = append(authorizedUserIDs, members...)
	}

	if authorizedUserIDs == nil {
		return errors.New("not a single user is authorized to respond to slack messages. check config")
	}

	s.authorizedUserIDs = authorizedUserIDs
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

func (s *slackClient) isUserAuthorized(userID string) bool {
	return util.StringSliceContains(s.authorizedUserIDs, userID)
}

func (s *slackClient) slackUserIDToName(userID string) (string, error) {
	user, err := s.slackClient.GetUserInfo(userID)
	if err != nil {
		return "", err
	}
	return user.Name, nil
}

func parseAlertFromSlackMessageText(text string) (map[string]string, error) {
	severityRegionRegex := regexp.MustCompile(SeverityRegionRegex)
	match := severityRegionRegex.FindStringSubmatch(text)
	matchMap := make(map[string]string)
	for i, name := range severityRegionRegex.SubexpNames() {
		if i > 0 && i <= len(match) {
			m := match[i]
			if name == "severity" || name == "region" {
				m = strings.ToLower(m)
				// 'warning - 2' -> warning
				if strings.Contains(m, "-") {
					s := strings.Split(m, "-")
					m = strings.TrimSpace(s[0])
				}
			}
			matchMap[name] = m
		}
	}

	if matchMap == nil || len(matchMap) == 0 {
		return nil, fmt.Errorf("no alert found in slack message: %s", text)
	}

	return matchMap, nil
}

func (s *slackClient) postToChannel(channel, message string) {
	s.slackClient.PostMessage(
		channel,
		message,
		slack.PostMessageParameters{
			Username: PostAsUserName,
		},
	)
}

func (s *slackClient) addReactionToMessage(message slack.Message, reaction string) {
	msgRef := slack.NewRefToMessage(message.Channel, message.Timestamp)
	if err := s.slackClient.AddReaction(reaction, msgRef); err != nil {
		log.Printf("error adding reaction to message: %v", err)
	}
}
