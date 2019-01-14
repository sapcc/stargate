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
	"fmt"
	"strings"

	"github.com/nlopes/slack"
	"github.com/nlopes/slack/slackevents"
	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	"github.com/sapcc/stargate/pkg/config"
	"github.com/sapcc/stargate/pkg/log"
	"github.com/sapcc/stargate/pkg/util"
)

// Client ...
type Client struct {
	config config.Config
	logger log.Logger

	// list of slack user ids that are authorized to interact with stargate messages
	authorizedUserIDs []string

	Client         *slack.Client
	slackRTMClient *slack.RTM
}

// NewClient returns a new slack client
func NewClient(config config.Config, opts config.Options, logger log.Logger) *Client {
	s := slack.New(config.Slack.AccessToken)
	s.SetDebug(opts.IsDebug)

	Client := &Client{
		config: config,
		logger: logger,
		Client: s,
	}

	logger = log.NewLoggerWith(logger, "component", "slack")

	if !config.Slack.IsDisableRTM {
		Client.slackRTMClient = NewSlackRTM(config, opts)
	}

	// get the list initially. refresh every slack.recheck_interval
	if err := Client.GetAuthorizedSlackUserGroupMembers(); err != nil {
		logger.LogFatal("failed to get authorized slack users", "err", err)
	}

	return Client
}

// AlertFromSlackMessage extracts an alert from a message
func (s *Client) AlertFromSlackMessage(message slack.Message) (*model.Alert, error) {
	text := messageTextFromSlack(message)
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

func messageTextFromSlack(message slack.Message) string {
	var text string
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
	return text
}

// GetAuthorizedSlackUserGroupMembers sets the authorized slack users based on the membership in slack groups
func (s *Client) GetAuthorizedSlackUserGroupMembers() error {
	userGroupIDs, err := s.userGroupNamesToIDs(s.config.Slack.AuthorizedGroups)
	if err != nil {
		return err
	}

	var authorizedUserIDs []string
	for _, groupID := range userGroupIDs {
		members, err := s.Client.GetUserGroupMembers(groupID)
		if err != nil {
			s.logger.LogError("error while getting members of group", err, "groupID", groupID)
			continue
		}
		authorizedUserIDs = append(authorizedUserIDs, members...)
	}

	if authorizedUserIDs == nil || len(authorizedUserIDs) == 0 {
		return errors.New("not a single user is authorized to respond to slack messages. check `authorized_groups` in config")
	}

	s.logger.LogInfo("authorizing members of slack users groups",
		"groups", strings.Join(s.config.Slack.AuthorizedGroups, ", "),
	)

	s.authorizedUserIDs = authorizedUserIDs
	return nil
}

// for convenience slack user groups are configured by name rather than ID.
// userGroupNamesToIDs finds the ID based on the name of a slack user group
func (s *Client) userGroupNamesToIDs(userGroupNames []string) ([]string, error) {
	userGroupIDs := make([]string, 0)
	userGroups, err := s.Client.GetUserGroups()
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

func (s *Client) IsUserAuthorized(userID string) bool {
	return util.StringSliceContains(s.authorizedUserIDs, userID)
}

// GetUserNameByID converts the userID to a human readable name in the format 'userRealName (userName)'.
func (s *Client) GetUserNameByID(userID string) (string, error) {
	user, err := s.Client.GetUserInfo(userID)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get slack info for user with id '%s'", userID)
	}

	var name string
	if user.RealName != "" && strings.ToUpper(user.RealName) != strings.ToUpper(user.Name) {
		name = user.RealName
	} else if user.Profile.DisplayName != "" && strings.ToUpper(user.Profile.DisplayName) != strings.ToUpper(user.Name) {
		name = user.Profile.DisplayName
	}

	if user.Name != "" {
		return fmt.Sprintf("%s (%s)", name, strings.ToUpper(user.Name)), nil
	}

	return name, nil
}

// GetUserEmailByID returns the users email address.
func (s *Client) GetUserEmailByID(userID string) (string, error) {
	userProfile, err := s.Client.GetUserProfile(userID, false)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get slack profile for user with id '%s'", userID)
	}

	email := userProfile.Email
	if email == "" {
		fmt.Errorf("user '%s' didn't maintain an email address", userProfile.RealName)
	}
	return email, nil
}

// PostMessage post a message to a channel.
// If 'timestamp' is given, the message is posted to a thread.
func (s *Client) PostMessage(channel, message, timestamp string) {
	postMessageParameters := slack.PostMessageParameters{
		Username:  s.config.Slack.UserName,
		LinkNames: 1,
	}
	if s.config.Slack.UserIcon != "" {
		postMessageParameters.IconEmoji = s.config.Slack.UserIcon
	}

	// respond to another message in an existing thread or create one
	if timestamp != "" {
		postMessageParameters.ThreadTimestamp = timestamp
	}

	_, _, err := s.Client.PostMessage(
		channel,
		message,
		postMessageParameters,
	)

	if err != nil {
		s.logger.LogError("error posting message to channel", err, "channel", channel)
	}
}

func (s *Client) AddReactionToMessage(channel, timestamp, reaction string) {
	s.logger.LogDebug("adding reaction to message", "reaction", reaction, "channel", channel, "timestamp", timestamp)
	msgRef := slack.NewRefToMessage(channel, timestamp)
	if err := s.Client.AddReaction(reaction, msgRef); err != nil {
		s.logger.LogError("error adding reaction to message", err, "channel", channel)
	}
}

// MessageActionFromPayload retrieves the slack message action from a payload
func (s *Client) MessageActionFromPayload(payload string) (slackevents.MessageAction, error) {
	slackMessageAction, err := slackevents.ParseActionEvent(
		payload,
		slackevents.OptionVerifyToken(&slackevents.TokenComparator{VerificationToken: s.config.Slack.GetValidationToken()}),
	)

	return slackMessageAction, err
}

// ActionFromSlackMessage retrieves the action from a slack message
func (s *Client) ActionFromSlackMessage(messageAction slackevents.MessageAction) ([]string, error) {
	reactions := make([]string, 0)
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
		reactions = append(reactions, action.Value)
	}
	return reactions, nil
}
