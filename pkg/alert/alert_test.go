/*******************************************************************************
*
* Copyright 2019 SAP SE
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

package alert

import (
	"github.com/prometheus/alertmanager/client"
	"github.com/prometheus/common/model"
	"github.com/sapcc/stargate/pkg/alertmanager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestAcknowledgeAlert(t *testing.T) {
	alertList := newAlerts()

	ackedAlertList := AcknowledgeAlerts(alertList, "Hans Glueck")
	// same person acked again. should not result in duplicates.
	ackedAlertList = AcknowledgeAlerts(ackedAlertList, "Peter")

	for _, alert := range ackedAlertList {
		ackedBy, ok := alert.Annotations[alertmanager.AcknowledgedByLabel]
		assert.True(t, ok, "the acknowledgedBy annotation should not be empty")
		assert.Equal(t, string(ackedBy), "Peter, Hans Glueck", "the the acknowledgedBy annotation should be equal")
	}
}

func newAlerts() []*client.ExtendedAlert {
	return []*client.ExtendedAlert{
		{
			Alert: client.Alert{
				Labels: client.LabelSet{
					model.AlertNameLabel: "alert1",
				},
				Annotations: client.LabelSet{
					alertmanager.AcknowledgedByLabel: "Peter",
				},
			},
		},
		{
			Alert: client.Alert{
				Labels: client.LabelSet{
					model.AlertNameLabel: "alert2",
				},
				Annotations: client.LabelSet{
					alertmanager.AcknowledgedByLabel: "Peter",
				},
			},
		},
	}
}

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
