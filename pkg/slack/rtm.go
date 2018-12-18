package slack

import (
	"fmt"
	"log"

	"github.com/nlopes/slack"
	"github.com/nlopes/slack/slackevents"
	"github.com/sapcc/stargate/pkg/config"
	"github.com/sapcc/stargate/pkg/util"
)

// NewSlackRTM ...
func NewSlackRTM(config config.Config, opts config.Options) *slack.RTM {
	client := slack.New(config.Slack.BotUserAccessToken)
	client.SetDebug(opts.IsDebug)
	return client.NewRTM()
}

// Run starts the slack RTM client
func (s *slackClient) RunRTM() {
	go s.slackRTMClient.ManageConnection()
	go s.HandleRTMEvent()
}

func (s *slackClient) HandleRTMEvent() {
	for msg := range s.slackRTMClient.IncomingEvents {
		switch event := msg.Data.(type) {

		// respond if the app was mentioned
		case *slackevents.AppMentionEvent:
			log.Println("app was mentioned. responding")

			region := parseRegionFromText(event.Text)
			action := parseActionFromText(event.Text)

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
					msg = fmt.Sprintf("Hey <@%s>, Relax! :green_heart:\nThere are no critical or warning alerts in %s.", event.User, region)
				} else {
					msg = fmt.Sprintf("Hey <@%s>, region %s shows:\n\n", event.User, region)
					msg += util.PrintableAlertDetails(alertsBySeverity)
				}

				s.postMessageToChannel(event.Channel, msg, "")
			}

			log.Printf("user: %s, channel: %s, text: %s", event.User, event.Channel, event.Text)

		case *slackevents.MessageEvent:
			log.Printf("received message event: %v", event)
		}
	}
}
