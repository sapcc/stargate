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
	"sync"
	"time"

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

// New creates a new stargate
func New(opts config.Options) *Stargate {
	cfg, err := config.NewConfig(opts)
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	sg := &Stargate{
		Config: cfg,
		slack:  slack.NewSlackClient(cfg, opts),
	}

	v1API := api.NewAPI(cfg)

	// the v1 endpoint that accepts slack message action events
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
	if err := r.ParseForm(); err != nil {
		log.Printf("failed to parse request from: %v", err)
		return
	}
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
func (s *Stargate) Run(wg *sync.WaitGroup, stopCh <-chan struct{}) {
	defer wg.Done()
	wg.Add(1)

	if !s.Config.Slack.IsDisableRTM {
		s.slack.RunRTM()
	}

	err := s.v1API.Serve()
	if err != nil {
		log.Fatal(err)
	}

	ticker := time.NewTicker(s.Config.Slack.RecheckInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := s.slack.GetAuthorizedSlackUserGroupMembers(); err != nil {
					log.Printf("error getting authorized slack user groups: %v", err)
				}
			case <-stopCh:
				ticker.Stop()
				return
			}
		}
	}()
	<-stopCh
}
