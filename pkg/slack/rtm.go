package slack

import (
	"fmt"
	"log"

	"github.com/nlopes/slack/slackevents"
	"github.com/sapcc/stargate/pkg/util"
  "github.com/sapcc/stargate/pkg/config"
  "github.com/nlopes/slack"
)

// NewSlackRTM ...
func NewSlackRTM(config config.Config) *slack.RTM {
  client := slack.New(config.SlackConfig.AccessToken)
  return client.NewRTM()
}

func (s *slackClient) HandleRTMEvent() {
	for msg := range s.slackRTMClient.IncomingEvents {
		fmt.Println("rtm event received")
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

		default:
			log.Printf("ignoring event type %v %v %v", msg.Type, msg.Data, event)
		}

	}
}
