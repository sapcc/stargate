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
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/nlopes/slack"
	"github.com/sapcc/stargate/pkg/util"
)

const (
	//RegionRegex is the regions by which regions names are found
	RegionRegex = `\S{2}\-\S{2}\-\d|staging|admin`
)

// Action ...
var Action = struct {
	ShowAlerts string
}{
	"showAlerts",
}

var commandActions = map[string][]string{
	Action.ShowAlerts: {"show", "alerts"},
}

// HandleSlackCommand responds to slack commands
func (s *slackClient) HandleSlackCommand(r *http.Request) {
	slashCommand, err := slack.SlashCommandParse(r)
	if err != nil {
		log.Print(err)
	}

	if !slashCommand.ValidateToken(s.config.SlackConfig.GetValidationToken()) {
		log.Printf("not authorized to perform command '%s'", slashCommand.Command)
		return
	}

	if slashCommand.Command == CommandCCloud {
		action := getCommandAction(slashCommand.Text)
		region := getCommandRegion(slashCommand.Text)

		if region == "" {
			s.postMessageToChannel(
				slashCommand.UserID,
				fmt.Sprintf("missing region. usage: %s %s <region>", slashCommand.Command, slashCommand.Text),
				"")
			return
		}

		switch action {
		case Action.ShowAlerts:
			alertList, err := s.alertmanagerClient.ListAlerts(map[string]string{"region": region})
			if err != nil {
				log.Printf("error listing alerts in region %s: %v", region, err)
			}

			alertsBySeverity, err := util.MapExtendedAlertsBySeverity(alertList)
			if err != nil {
				log.Println(err)
				return
			}

			var msg string
			if util.IsNoCriticalOrWarningAlerts(alertsBySeverity) {
				msg = fmt.Sprintf("Hey <@%s>, Relax! :green_heart:\nThere are no critical or warning alerts in %s.", slashCommand.UserID, region)
			} else {
				msg = fmt.Sprintf("Hey <@%s>, region %s shows:\n\n", slashCommand.UserID, region)
				msg += util.PrintableExtendedAlertsBySeverity(alertsBySeverity)
			}

			s.postMessageToChannel(slashCommand.ChannelID, msg, "")
		}
	}
}

func getCommandRegion(cmdText string) string {
	r := regexp.MustCompile(RegionRegex)
	return r.FindString(strings.ToLower(cmdText))
}

func getCommandAction(cmdText string) string {
	for cmd, keywords := range commandActions {
		if textContainsAllKeyWords(cmdText, keywords) {
			return cmd
		}
	}
	return ""
}

func textContainsAllKeyWords(text string, keywords []string) bool {
	for _, k := range keywords {
		if !strings.Contains(text, k) {
			return false
		}
	}
	return true
}
