package util

import (
	"testing"
	"time"

	"github.com/prometheus/alertmanager/client"
	"github.com/stretchr/testify/require"
)

func TestPrintableAlertSummary(t *testing.T) {
	summary := PrintableAlertSummary(getDummyAlertBySeverity())
	require.NotEmpty(t, summary)
	t.Log(summary)
}

func TestPrintableAlertDetails(t *testing.T) {
	details := PrintableAlertDetails(getDummyAlertBySeverity())
	require.NotEmpty(t, details)
	t.Log(details)
}

func getDummyAlertBySeverity() map[string][]*client.ExtendedAlert {
	return map[string][]*client.ExtendedAlert{
		AlertSeverity.Critical: {
			{
				Alert: client.Alert{
					Labels: map[client.LabelName]client.LabelValue{
						client.LabelName("alertname"): client.LabelValue("KubernetesManyNodeDown"),
						client.LabelName("severity"):  client.LabelValue("critical"),
						client.LabelName("region"):    client.LabelValue("staging"),
					},
					StartsAt: time.Now().UTC().AddDate(0, 0, -1),
				},
			},
			{
				Alert: client.Alert{
					Labels: map[client.LabelName]client.LabelValue{
						client.LabelName("alertname"):      client.LabelValue("KubernetesManyAPIDown"),
						client.LabelName("severity"):       client.LabelValue("critical"),
						client.LabelName("region"):         client.LabelValue("staging"),
						client.LabelName("acknowledgedBy"): client.LabelValue("Max S."),
					},
					StartsAt: time.Now().UTC().AddDate(0, -1, -1),
				},
			},
		},
		AlertSeverity.Warning: {
			{
				Alert: client.Alert{
					Labels: map[client.LabelName]client.LabelValue{
						client.LabelName("alertname"): client.LabelValue("KubernetesNodeDown"),
						client.LabelName("severity"):  client.LabelValue("warning"),
						client.LabelName("region"):    client.LabelValue("staging"),
						client.LabelName("meta"):      client.LabelValue("node master0.kubernetes.cluster.local is down"),
					},
					StartsAt: time.Now().UTC().AddDate(-1, 0, 0),
				},
			},
			{
				Alert: client.Alert{
					Labels: map[client.LabelName]client.LabelValue{
						client.LabelName("alertname"): client.LabelValue("KubernetesAPIDown"),
						client.LabelName("severity"):  client.LabelValue("warning"),
						client.LabelName("region"):    client.LabelValue("staging"),
						client.LabelName("service"):   client.LabelValue("kubernetes"),
					},
					StartsAt: time.Now().UTC().AddDate(0, 0, -14),
				},
			},
		},
		AlertSeverity.Info: {
			{
				Alert: client.Alert{
					Labels: map[client.LabelName]client.LabelValue{
						client.LabelName("alertname"): client.LabelValue("KubernetesPVCPending"),
						client.LabelName("severity"):  client.LabelValue("info"),
						client.LabelName("region"):    client.LabelValue("staging"),
					},
				},
			},
		},
	}
}
