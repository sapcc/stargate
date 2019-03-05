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

### Internal Endpoints

The following endpoints might be useful for testing and debugging.

#### GET `/api/v1/-/store/alerts`

The v1 endpoint that lists alerts currently held in the internal alert store.

#### POST `/api/v1/-/store/acknowledge`

The v1 endpoint that acknowledges an alert.
Example:
```
curl -u "<username>:<password>" \
    -d '{"data": {"alertname": "<alertname>", "region": "<region>", "acknowledgedBy": "Test user"}}' \
    -H "Content-Type: application/json" \
    -X POST \
    https://stargate.eu-de-2.cloud.sap/api/v1/-/store/acknowledge
```

#### GET `/api/v1/-/alertmanager/alerts`

The v1 endpoint that lists alerts from the alertmanager.

#### GET `/api/v1/-/pagerduty/incidents`

The v1 endpoint that lists incidents from pagerduty.
