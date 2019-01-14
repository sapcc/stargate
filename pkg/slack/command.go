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
	"net/http"

	"github.com/nlopes/slack"
	"github.com/sapcc/stargate/pkg/util"
)

// HandleSlackCommand responds to slack commands
func (s *Client) HandleSlackCommand(r *http.Request) {
	slashCommand, err := slack.SlashCommandParse(r)
	if err != nil {
		s.logger.LogError("failed to parse slash command", err)
	}

	if !slashCommand.ValidateToken(s.config.Slack.GetValidationToken()) {
		s.logger.LogInfo("failed to validate token for slash command")
		return
	}

	if slashCommand.Command == s.config.Slack.Command {
		action := parseActionFromText(slashCommand.Text)
		region := parseRegionFromText(slashCommand.Text)

		if region == "" {
			s.PostMessage(
				slashCommand.UserID,
				fmt.Sprintf("missing region. usage: %s %s <region>", slashCommand.Command, slashCommand.Text),
				"")
			return
		}

		switch action {
		case Action.ShowAlerts:
			alertList, err := s.alertmanagerClient.ListAlerts(map[string]string{"region": region})
			if err != nil {
				s.logger.LogError("error listing alerts in region", err, "region", region)
			}

			alertsBySeverity, err := util.MapExtendedAlertsBySeverity(alertList)
			if err != nil {
				s.logger.LogError("", err)
				return
			}

			var msg string
			if util.IsNoCriticalOrWarningAlerts(alertsBySeverity) {
				msg = fmt.Sprintf("Hey <@%s>, Relax! :green_heart:\nThere are no critical or warning alerts in %s.", slashCommand.UserID, region)
			} else {
				msg = fmt.Sprintf("Hey <@%s>, region %s shows:\n\n", slashCommand.UserID, region)
				msg += util.PrintableAlertDetails(alertsBySeverity)
			}

			s.PostMessage(slashCommand.ChannelID, msg, "")
		}
	}
}
