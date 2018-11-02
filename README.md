# Stargate

[![Build Status](https://travis-ci.org/sapcc/stargate.svg?branch=master)](https://travis-ci.org/sapcc/stargate)
[![Docker Repository](https://img.shields.io/docker/pulls/sapcc/stargate.svg?maxAge=604800)](https://hub.docker.com/r/sapcc/stargate/)

The Stargate opens a gate from [Slack](https://slack.com) to the several systems to enable interaction with these.
The primary gate opens to the [Prometheus Alertmanager](https://prometheus.io/docs/alerting/alertmanager) so one can respond to alerts from Slack.

## Features

- Respond to Prometheus alerts from the slack messenger
- Silence alerts in the Prometheus Alertmanager using interactive slack messages

Currently, the stargate only supports **Slack** as a messenger and the **Prometheus Alertmanager** as receiver.
Interactive messages are `POST`ed to `/v1/slack` where they are verified using either the `signing_secret` or `verification_token`.

## Configuration

Minimal configuration:
```yaml

slack:
  # provide signing_secret or verification token to verify messages sent by slack
  signing_secret:       <string>
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
```

## Installation & Usage

```
Usage of the stargate:
      --alertmanager-url string     URL of the Prometheus Alertmanager
      --config-file string                 Path to the file containing the config (default "/etc/stargate/config/stargate.yaml")
      --debug                                   Enable debug configuration and log level
      --external-url string              External URL
      --port int                                 API port (default 8080)

```

A helm chart is provided in the [helm-charts folder](./helm).

