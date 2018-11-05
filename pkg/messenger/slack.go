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
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/nlopes/slack"
	"github.com/nlopes/slack/slackevents"
	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	"github.com/sapcc/stargate/pkg/alertmanager"
	"github.com/sapcc/stargate/pkg/config"
	"github.com/sapcc/stargate/pkg/util"
)

const (
	// DurationOneDay is a day in hours
	DurationOneDay = 24 * time.Hour

	// Silence8h action type
	Silence8h = "silence8h"

	// Silence7d action type
	Silence7d = "silence7d"

	// Silence1month action type
	Silence1month = "silence1month"

	// ActionName the name of the action the stargate is responding to
	ActionName = "reaction"

	// ActionType the type of the action the stargate is responding to
	ActionType = "button"

	// Acknowledge action type
	Acknowledge = "acknowledge"

	// PostAsUserName appears in the messenger message
	PostAsUserName = "stargate"

	// SilenceDefaultAuthor is the default author used for a silence
	SilenceDefaultAuthor = "stargate"

	// SilenceSuccessReactionEmoji is applied to a message after it was successfully silenced
	SilenceSuccessReactionEmoji = "silent-bell"

	// SilenceDefaultComment is the default comment used for a silence
	SilenceDefaultComment = "silenced by the stargate"
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
		log.Fatalf("failed to get authorized slack users: %v", err)
	}

	return slackClient
}

// HandleMessage parses the payload and immediately returns 204 while actually handling the request in the background
func (s *slackClient) HandleMessage(w http.ResponseWriter, r *http.Request) {
	log.Println("received slack message")
	w.WriteHeader(http.StatusNoContent)
	r.ParseForm()
	var payloadString string
	for k, v := range r.Form {
		if k == "payload" && len(v) == 1 {
			payloadString = v[0]
			break
		}
	}

	go s.handleSlackMessage(payloadString)

	return
}

func (s *slackClient) handleSlackMessage(payloadString string) {
	if payloadString == "" {
		log.Printf("empty paylod. request does not contain a slack message action event")
		return
	}

	slackMessageAction, err := slackevents.ParseActionEvent(
		payloadString,
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

func (s *slackClient) checkAction(messageAction slackevents.MessageAction) error {
	for _, action := range messageAction.Actions {
		// only react to buttons clicks
		if action.Name != ActionName || action.Type != ActionType {
			log.Printf("ignoring action with name '%s', type '%s', value '%s'", action.Name, action.Type, action.Value)
			continue
		}

		switch action.Value {
		case Silence8h:
			if err := s.createSilence(messageAction, 8*time.Hour); err != nil {
				log.Printf("error creating silence: %v", err)
			}
		case Silence7d:
			if err := s.createSilence(messageAction, 7*DurationOneDay); err != nil {
				log.Printf("error creating silence: %v", err)
			}
		case Silence1month:
			if err := s.createSilence(messageAction, 31*DurationOneDay); err != nil {
				log.Printf("error creating silence: %v", err)
			}

		default:
			log.Printf("not responding to action '%s'", action.Value)
		}
	}

	return nil
}

func (s *slackClient) createSilence(messageAction slackevents.MessageAction, duration time.Duration) error {
	alert, err := s.alertFromSlackMessage(messageAction.OriginalMessage)
	if err != nil {
		return errors.Wrapf(err, "failed to construct alert from slack message")
	}

	userName, err := s.slackUserIDToName(messageAction.User.Id)
	if err != nil {
		log.Printf("error finding slack user by id: %v", err)
		userName = SilenceDefaultAuthor
	}

	silenceID, err := s.alertmanagerClient.CreateSilence(
		alert,
		userName,
		SilenceDefaultComment,
		duration,
	)
	if err != nil {
		return err
	}

	// Confirm the silence was successfully created by posting to the channel
	s.addReactionToMessage(messageAction.Channel.Id, messageAction.OriginalMessage.Timestamp, SilenceSuccessReactionEmoji)

	// Confirm the silence was successfully created by responding to the original message
	return s.postMessageToChannel(
		messageAction.Channel.Id,
		fmt.Sprintf("%s silenced alert %s for %s. <%s|Silence>", userName, alert.Name(), util.HumanizedDurationString(duration), s.alertmanagerClient.LinkToSilence(silenceID)),
		messageAction.OriginalMessage.Timestamp,
	)
}

func (s *slackClient) alertFromSlackMessage(message slack.Message) (*model.Alert, error) {
	var text string
	// sometimes it's message.Text
	if message.Text != "" {
		text = message.Text
		// sometimes it's in the attachment
	} else if message.Attachments != nil && len(message.Attachments) > 0 {
		for _, attach := range message.Attachments {
			if attach.Text != "" {
				text = attach.Text
				break
			}
		}
	}

	labels, err := parseAlertFromSlackMessageText(text)
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
	log.Printf(
		"authorizing members of slack users groups: %v",
		strings.Join(s.config.SlackConfig.AuthorizedGroups, ", "),
	)

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

	if authorizedUserIDs == nil || len(authorizedUserIDs) == 0 {
		return errors.New("not a single user is authorized to respond to slack messages. check `authorized_groups` in config")
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

// slackUserIDToName converts the userID to a human readable name in the format 'userRealName (userName)'
func (s *slackClient) slackUserIDToName(userID string) (string, error) {
	user, err := s.slackClient.GetUserInfo(userID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s (%s)", user.RealName, strings.ToUpper(user.Name)), nil
}

func (s *slackClient) postMessageToChannel(channel, message, threadTimestamp string) error {
	postMessageParameters := slack.PostMessageParameters{
		Username: PostAsUserName,
	}

	// respond to another message in an existing thread or create one
	if threadTimestamp != "" {
		postMessageParameters.ThreadTimestamp = threadTimestamp
	}

	_, _, err := s.slackClient.PostMessage(
		channel,
		message,
		postMessageParameters,
	)
	return err
}

func (s *slackClient) addReactionToMessage(channel, timestamp, reaction string) {
	log.Printf("adding reaction '%s' to message with channel '%s', timestamp '%s", reaction, channel, timestamp)
	msgRef := slack.NewRefToMessage(channel, timestamp)
	if err := s.slackClient.AddReaction(reaction, msgRef); err != nil {
		log.Printf("error adding reaction to message: %v", err)
	}
}
