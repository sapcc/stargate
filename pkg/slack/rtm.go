package slack

import (
	"fmt"

	"github.com/nlopes/slack"
	"github.com/nlopes/slack/slackevents"
	"github.com/sapcc/stargate/pkg/alert"
	"github.com/sapcc/stargate/pkg/alertmanager"
	"github.com/sapcc/stargate/pkg/config"
)

// NewSlackRTM ...
func NewSlackRTM(config config.Config, opts config.Options) *slack.RTM {
	client := slack.New(config.Slack.BotUserAccessToken)
	client.SetDebug(opts.IsDebug)
	return client.NewRTM()
}

// RunRTM starts the slack RTM client
func (s *Client) RunRTM() {
	s.logger.LogInfo("starting slack real time messaging")
	go s.slackRTMClient.ManageConnection()
	go s.HandleRTMEvent()
}

// HandleRTMEvent handles slack RTM events
func (s *Client) HandleRTMEvent() {
	for msg := range s.slackRTMClient.IncomingEvents {
		switch event := msg.Data.(type) {

		// respond if the app was mentioned
		case *slackevents.AppMentionEvent:
			s.logger.LogDebug("app was mentioned. responding")

			region := parseRegionFromText(event.Text)
			action := parseActionFromText(event.Text)

			switch action {
			case Action.ShowAlerts:
				filter := alertmanager.NewDefaultFilter()
				filter.WithAdditionalFilter(map[string]string{"region": region})

				alertList, err := s.alertmanagerClient.ListAlerts(filter)
				if err != nil {
					s.logger.LogError("error listing alerts", err, "region", region)
				}

				alertsBySeverity, err := alert.MapExtendedAlertsBySeverity(alertList)
				if err != nil {
					s.logger.LogError("error mapping alerts by severity", err)
					return
				}

				var msg string
				if alert.IsNoCriticalOrWarningAlerts(alertsBySeverity) {
					msg = fmt.Sprintf("Hey <@%s>, Relax! :green_heart:\nThere are no critical or warning alerts in %s.", event.User, region)
				} else {
					msg = fmt.Sprintf("Hey <@%s>, region %s shows:\n\n", event.User, region)
					msg += alert.PrintableAlertDetails(alertsBySeverity)
				}

				s.PostMessage(event.Channel, msg, "")
			}

			s.logger.LogDebug("responding to action", "user", event.User, "channel", event.Channel, "text", event.Text)

		case *slackevents.MessageEvent:
			s.logger.LogDebug("received message event")

		case *slackevents.MessageAction:
			s.logger.LogDebug("received message action")
		}
	}
}
