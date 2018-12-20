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
	"time"

	"github.com/nlopes/slack"
	"github.com/nlopes/slack/slackevents"
	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	"github.com/sapcc/stargate/pkg/alertmanager"
	"github.com/sapcc/stargate/pkg/config"
	"github.com/sapcc/stargate/pkg/log"
	"github.com/sapcc/stargate/pkg/pagerduty"
	"github.com/sapcc/stargate/pkg/util"
)

type slackClient struct {
	config config.Config
	logger log.Logger

	// list of slack user ids that are authorized to interact with stargate messages
	authorizedUserIDs []string

	slackClient        *slack.Client
	slackRTMClient     *slack.RTM
	alertmanagerClient alertmanager.Alertmanager
	pagerdutyClient    *pagerduty.Client
}

// NewSlackClient returns a new receiver
func NewSlackClient(config config.Config, opts config.Options, logger log.Logger) Receiver {
	s := slack.New(config.Slack.AccessToken)
	s.SetDebug(opts.IsDebug)

	slackClient := &slackClient{
		config:             config,
		logger:             logger,
		alertmanagerClient: alertmanager.New(config, logger),
		slackClient:        s,
		pagerdutyClient:    pagerduty.NewClient(config, logger),
	}

	logger = log.NewLoggerWith(logger, "component", "slack")

	if !config.Slack.IsDisableRTM {
		slackClient.slackRTMClient = NewSlackRTM(config, opts)
	}

	// get the list initially. refresh every slack.recheck_interval
	if err := slackClient.GetAuthorizedSlackUserGroupMembers(); err != nil {
		logger.LogFatal("failed to get authorized slack users", "err", err)
	}

	return slackClient
}

// acknowledgeAlert acknowledges an alert
func (s *slackClient) acknowledgeAlert(messageAction slackevents.MessageAction) error {
	alert, err := s.alertFromSlackMessage(messageAction.OriginalMessage)
	if err != nil {
		return errors.Wrapf(err, "failed to construct alert from slack message")
	}

	// get human readable user name
	userName, err := s.slackUserIDToName(messageAction.User.Id)
	if err != nil {
		s.logger.LogError("error finding slack user by id", err)
		userName = s.config.Slack.UserName
	}

	// acknowledge alert in the alertmanager
	if err := s.alertmanagerClient.AcknowledgeAlert(alert, userName); err != nil {
		s.logger.LogError("failed to acknowledge in alertmanager", err)
	}

	// get user mail address. req. for pagerduty acknowledgements
	userEMail, err := s.getUserEmail(messageAction.User.Id)
	if err != nil {
		s.logger.LogError("failed to get user email address", err)
	}

	// acknowledge alert in pagerduty
	if err := s.pagerdutyClient.AcknowledgeIncident(alert, userEMail); err != nil {
		s.logger.LogError("failed to acknowledge in pagerduty", err)
	}

	s.addReactionToMessage(
		messageAction.Channel.Id,
		messageAction.OriginalMessage.Timestamp,
		AcknowledgeReactionEmoji,
	)

	return s.postMessageToChannel(
		messageAction.Channel.Id,
		fmt.Sprintf("Acknowledged by <@%s>", messageAction.User.Id),
		messageAction.OriginalMessage.Timestamp,
	)
}

// silenceAlert extracts the alert from a text message and creates an alert for it
func (s *slackClient) silenceAlert(messageAction slackevents.MessageAction, duration time.Duration) error {
	alert, err := s.alertFromSlackMessage(messageAction.OriginalMessage)
	if err != nil {
		return errors.Wrapf(err, "failed to construct alert from slack message")
	}

	userName, err := s.slackUserIDToName(messageAction.User.Id)
	if err != nil {
		s.logger.LogWarn("error finding slack user by id. using default username", "err", err)
		userName = s.config.Slack.UserName
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
		fmt.Sprintf("<@%s> silenced alert %s for %s. <%s|See Silence>", messageAction.User.Id, alert.Name(), util.HumanizedDurationString(duration), s.alertmanagerClient.LinkToSilence(silenceID)),
		messageAction.OriginalMessage.Timestamp,
	)
}

// alertFromSlackMessage extracts an alert from a message
func (s *slackClient) alertFromSlackMessage(message slack.Message) (*model.Alert, error) {
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

func isErrorInvalidToken(err error) bool {
	if err.Error() == "invalid verification token" {
		return true
	}
	return false
}

func (s *slackClient) GetAuthorizedSlackUserGroupMembers() error {
	userGroupIDs, err := s.userGroupNamesToIDs(s.config.Slack.AuthorizedGroups)
	if err != nil {
		return err
	}

	var authorizedUserIDs []string
	for _, groupID := range userGroupIDs {
		members, err := s.slackClient.GetUserGroupMembers(groupID)
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

func (s *slackClient) getUserEmail(userID string) (string, error) {
	userProfile, err := s.slackClient.GetUserProfile(userID, false)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get slack profile for user with id '%s'", userID)
	}

	email := userProfile.Email
	if email == "" {
		fmt.Errorf("user '%s' didn't maintain an email address", userProfile.RealName)
	}
	return email, nil
}

func (s *slackClient) postMessageToChannel(channel, message, threadTimestamp string) error {
	postMessageParameters := slack.PostMessageParameters{
		Username:  s.config.Slack.UserName,
		LinkNames: 1,
	}
	if s.config.Slack.UserIcon != "" {
		postMessageParameters.IconEmoji = s.config.Slack.UserIcon
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
	s.logger.LogDebug("adding reaction to message", "reaction", reaction, "channel", channel, "timestamp", timestamp)
	msgRef := slack.NewRefToMessage(channel, timestamp)
	if err := s.slackClient.AddReaction(reaction, msgRef); err != nil {
		s.logger.LogError("error adding reaction to message", err)
	}
}
