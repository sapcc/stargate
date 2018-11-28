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

package stargate

import (
	"log"
	"net/http"

	"github.com/sapcc/stargate/pkg/alertmanager"
	"github.com/sapcc/stargate/pkg/api"
	"github.com/sapcc/stargate/pkg/config"
	"github.com/sapcc/stargate/pkg/slack"
)

// Stargate ...
type Stargate struct {
	v1API *api.API

	alertmanagerClient alertmanager.Alertmanager
	slack              slack.Receiver

	Config config.Config
}

// NewStargate creates a new stargate
func NewStargate(opts config.Options) *Stargate {
	cfg, err := config.NewConfig(opts)
	if err != nil {
		log.Fatal(err)
	}

	sg := &Stargate{
		Config: cfg,
		slack:  slack.NewSlackClient(cfg, opts.IsDebug),
	}

	v1API := api.NewAPI(cfg)

	// the v1 endpoint that accepts slack message actions
	v1API.AddRouteV1(http.MethodPost, "/slack/event", sg.HandleSlackMessageActionEvent)

	// the v1 endpoint that accepts slack commands
	v1API.AddRouteV1(http.MethodPost, "/slack/command", sg.HandleSlackCommand)

	sg.v1API = v1API
	return sg
}

// HandleSlackMessageActionEvent handles slack message action events
func (s *Stargate) HandleSlackMessageActionEvent(w http.ResponseWriter, r *http.Request) {
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

	go s.slack.HandleSlackMessageActionEvent(payloadString)
}

// HandleSlackCommand handles slack commands
func (s *Stargate) HandleSlackCommand(w http.ResponseWriter, r *http.Request) {
	log.Println("received slack command")
	w.WriteHeader(http.StatusNoContent)
	r.ParseForm()

	go s.slack.HandleSlackCommand(r)
}

// Run starts the stargate
func (s *Stargate) Run() {
	err := s.v1API.Serve()
	if err != nil {
		log.Fatal(err)
	}
}
