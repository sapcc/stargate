# Installation

This sections describes the necessary steps to setup the stargate.

## Slack

In Slack create a new App with the following scopes:
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

Generate incoming webhooks in the Slack app for each channel to which the Stargate should post. A generic incoming webhook with channel override will not work.

## Stargate

The Stargate comes with a [helm-chart](./helm) to ease the installation in a Kubernetes environment.  
To install the Stargate simply run
```
helm upgrade stargate ./helm --namespace=stargate --values <path to values.yaml> --install
```

The Stargate provides the following flags:

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
