/*******************************************************************************
*
* Copyright 2018 SAP SE
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

package messenger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	slackMessageTextSingleAlert   = "n*[CRITICAL]* *[STAGING]* *<https://alertmanager.tld/#/alerts?receiver=slack_general|OpenstackManilaDatapathDown>* - Datapath manila nfs is downnn:fire: Datapath manila nfs is down for 15 minutes. See Sentry for details (<https://prometheus.tld/graph?g0.expr=blackbox_datapath_status_gauge%7Bservice%3D~%22manila%22%7D+%3D%3D+1&amp;g0.tab=1|Graph>)n*<https://grafana.tld/dashboard/db/ccloud-health-datapath-details|Grafana>* *<https://sentry.tld/monsoon/blackbox/?query=test_nfs|Sentry>* *<https://operations.tld/docs/devops/alert/manila/#nfs|Playbook>*"
	slackMessageTextMultiAlert    = "n*[WARNING - 2]* *[STAGING]* *<https://alertmanager.tld/#/alerts?receiver=slack_general|OpenstackCinderCanaryDown>* - nn:warning: Canary cinder create_volume-staginga is down for 1 hour. See Sentry for details (<https://prometheus.tld/graph?g0.expr=blackbox_canary_status_gauge%7Bservice%3D~%22cinder%22%7D+%3D%3D+1&amp;g0.tab=1|Graph>)n:warning: Canary cinder create_volume-stagingb is down for 1 hour. See Sentry for details (<https://prometheus.tld/graph?g0.expr=blackbox_canary_status_gauge%7Bservice%3D~%22cinder%22%7D+%3D%3D+1&amp;g0.tab=1|Graph>)n*<https://grafana.tld/dashboard/db/ccloud-health-canary-details|Grafana>* *<https://operations.tld/docs/devops/alert/cinder|Playbook>* "
	slackMessageTextResolvedAlert = "n*[RESOLVED]* *[STAGING]* *<https://alertmanager.tld/#/alerts?receiver=slack_general|OpenstackLimesMissingCapacity>* - Limes reports zero capacity for volumev2/capacitynn:white_check_mark: Limes reports no capacity for volumev2/capacity. This usually means that the backend service reported weirdly-shaped data to Limes' capacity scanner. The log for limes-collect-ccloud may contain additional info.n*<https://grafana.tld/dashboard/db/limes-overview|Grafana>* "
)

func TestParseSlackMessageTextSingleAlert(t *testing.T) {
	actualMatchMap, err := parseAlertFromSlackMessageText(slackMessageTextSingleAlert)

	assert.NoError(t, err, "parsing the severity, region, alertname should not throw an error")
	assert.NotEmpty(t, actualMatchMap, "should have found the severity, region, alertname in the test string")

	expectedMatchMap := map[string]string{
		"alertname": "OpenstackManilaDatapathDown",
		"region":    "staging",
		"severity":  "critical",
	}

	assert.Equal(t, expectedMatchMap, actualMatchMap)
}

func TestParseSlackMessageTextMultipleAlerts(t *testing.T) {
	actualMatchMap, err := parseAlertFromSlackMessageText(slackMessageTextMultiAlert)

	assert.NoError(t, err, "parsing the severity, region, alertname should not throw an error")
	assert.NotEmpty(t, actualMatchMap, "should have found the severity, region, alertname in the test string")

	expectedMatchMap := map[string]string{
		"alertname": "OpenstackCinderCanaryDown",
		"region":    "staging",
		"severity":  "warning",
	}

	assert.Equal(t, expectedMatchMap, actualMatchMap)
}

func TestParseSlackMessageTextResolvedAlert(t *testing.T) {
	_, err := parseAlertFromSlackMessageText(slackMessageTextResolvedAlert)
	assert.EqualError(
		t,
		err,
		"ignoring resolved message",
		"should throw an error as resolved messages are ignored",
	)
}
