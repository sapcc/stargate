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

## Installation, Configuration, API

See the [installation guide](./docs/install.md) as well as the [API documentation](./docs/api.md).

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


See the [full Stargate configuration example](./etc/stargate.yaml).
Also see the [Prometheus Alertmanager configuration](./etc/alertmanager.yaml) and the corresponding [Slack template](./etc/slack.tmpl).
Full example via helm chart is available [here](https://github.com/sapcc/helm-charts/tree/master/global/prometheus-alertmanager).
