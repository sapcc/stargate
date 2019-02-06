# Stargate

[![Build Status](https://travis-ci.org/sapcc/stargate.svg?branch=master)](https://travis-ci.org/sapcc/stargate)
[![Docker Repository](https://img.shields.io/docker/pulls/sapcc/stargate.svg?maxAge=604800)](https://hub.docker.com/r/sapcc/stargate/)

The Stargate opens a gate from [Slack](https://slack.com) to the several systems to enable interaction with these :)
The primary gate opens to the [Prometheus Alertmanager](https://prometheus.io/docs/alerting/alertmanager) so one can respond to alerts from Slack.

## Features

- Respond to Prometheus alerts from the Slack messenger.
- Silence alerts in the Prometheus Alertmanager using interactive Slack messages.
- Acknowledge alerts in the Alertmanager and incidents in Pagerduty using interactive Slack messages.
- Visualize acknowledged and silenced alerts in [Grafana](https://grafana.com/) using the Stargate and the [Prometheus Alertmanager datasource](https://github.com/sapcc/grafana-prometheus-alertmanager-datasource).

Currently, the stargate only supports **Slack** as a messenger and the **Prometheus Alertmanager**, **Pagerduty** as receiver.

## Configuration

See the [full Stargate configuration example](./etc/stargate.yaml).
Also see the [Prometheus Alertmanager configuration](./etc/alertmanager.yaml) and the corresponding [Slack template](./etc/slack.tmpl).
Full example via helm chart is available [here](https://github.com/sapcc/helm-charts/tree/master/global/prometheus-alertmanager).

Minimal configuration of the Stargate:
```yaml

slack:
  # provide signing_secret or verification token to verify messages sent by slack
  verification_token:   <string>

  # stargate oauth token. required
  access_token:         <string>

  # members of these slack user groups are authorized to interact with slack messages
  authorized_groups:
    - admin
    - ...

alertmanager:
  # URL of the Prometheus Alertmanager
  url: <string>
```

The stargate requires the following slack scopes:
```
- incoming-webhook
- usergroups:read
- users:read
- users.profile:read
- reactions:write
- chat:write:bot
- chat:write:user
- rtm:stream
```

## Installation & Usage

```
Usage of stargate:
      --config-file string              Path to the file containing the config (default "/etc/stargate/config/stargate.yaml")
      --debug                           Enable debug configuration and log level
      --disable-slack-rtm               Disable Slack RTM (the bot)
      --external-url string             External URL
      --metric-port int                 Metric port (default 9090)
      --persistence-file string         Path to the file used to persist the alert store (default "/data/alerts.dump")
      --port int                        API port (default 8080)
      --recheck-interval duration       Garbage collections within the alert store happens that often (default 5m0s)
```

A helm chart is provided in the [helm-charts folder](./helm).


## API

This section lists the available endpoints of the Stargates v1 API.

### Slack endpoints

Interactive Slack messages are `POST`ed to the following endpoints where they are verified using the `verification_token`.
Given the verification was successful, the message is parsed for an alert and handled according to which button was clicked.

#### POST `/api/v1/slack/event`

The v1 endpoint that accepts slack message action events.
Configure this in your Slack application.

#### POST `/api/v1/slack/command`

The v1 endpoint that accepts slack commands.
Configure this in your Slack application.

### Endpoints

The following endpoints can be used to visualize the current alert situation in a Grafana dashboard.
The [Prometheus Alertmanager datasource](https://github.com/sapcc/grafana-prometheus-alertmanager-datasource) needs to be installed first pointing
to the Stargate. Use Server access mode.
Basic authentication is required using the Slack `user_name` and `signing_secret` or alternatively `verification_token`.

#### GET `/api/v1/slack/status`

The v1 endpoint that shows the status of the stargate.
Required by Grafana to test the datasource.

#### GET `/api/v1/slack/alerts`

The v1 endpoint that lists the alerts.
Alerts can be filtered via query.
See Prometheus Alertmanager API documentation for more details.

#### GET `/api/v1/slack/silences`

The v1 endpoint that list silences.
Silences can be filtered via query.
See Prometheus Alertmanager API documentation for more details.

#### GET `/api/v1/slack/silence/{silenceID}`

The v1 endpoint that gets a silence its `silenceID`.
