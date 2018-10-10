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
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/nlopes/slack"
	"github.com/nlopes/slack/slackevents"
	"github.com/prometheus/common/model"
	"github.com/sapcc/stargate/pkg/alertmanager"
	"github.com/sapcc/stargate/pkg/api"
	"github.com/sapcc/stargate/pkg/config"
)

const (
	// Silence action type
	Silence = "silence"

	// Acknowledge action type
	Acknowledge = "acknowledge"

)

type slackReceiver struct {
	config config.Config

	alertmanagerClient alertmanager.Alertmanager
}

// New returns a new receiver
func New(config config.Config) Receiver {
	return &slackReceiver{
		config:             config,
		alertmanagerClient: alertmanager.New(config),
	}
}

// HandleMessage handles a slack message
func (s *slackReceiver) HandleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		json.NewEncoder(w).Encode(
			api.Error{
				Code:    405,
				Message: "This endpoint only supports POST",
			},
		)
		return
	}

	log.Println("received slack slack")

	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Body)

	slackMessageAction, err := slackevents.ParseActionEvent(
		buf.String(),
		slackevents.OptionVerifyToken(&slackevents.TokenComparator{VerificationToken: "TOKEN"}),
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

	for _, action := range slackMessageAction.Actions {
		// only react to buttons clicks
		if action.Name != "reaction" {
			continue
		}
		switch action.Value {
		case Silence:
			//TODO:
			continue

		default:
			log.Printf("not responding to action '%s'", action.Value)
		}
	}
}

func (s *slackReceiver) checkAction(messageAction slackevents.MessageAction) {
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
		default:
			log.Printf("not responding to action '%s'", action.Value)
		}
	}
}

//TODO: which fields does the slack slack contain
func (s *slackReceiver) alertFromSlackMessage(message slack.Message) *model.Alert {
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
