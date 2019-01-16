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
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/prometheus/alertmanager/client"
	"github.com/prometheus/common/model"
	"github.com/sapcc/stargate/pkg/alertmanager"
	"github.com/sapcc/stargate/pkg/util"
)

// AlertSeverity ...
var AlertSeverity = struct {
	Critical,
	Warning,
	Info string
}{
	"critical",
	"warning",
	"info",
}

// AcknowledgeAlert sets the acknowledgedBy annotation for an alert
func AcknowledgeAlert(alert *client.ExtendedAlert, acknowledgedBy string) *client.ExtendedAlert {
	clone := cloneAlert(alert)

	if clone.Annotations == nil {
		clone.Annotations = client.LabelSet{}
	}

	ack, ok := clone.Annotations[alertmanager.AcknowledgedByLabel]
	if !ok {
		clone.Annotations[alertmanager.AcknowledgedByLabel] = client.LabelValue(acknowledgedBy)
		return clone
	}

	// alert already acked by this person.
	if strings.Contains(string(ack), acknowledgedBy) {
		return clone
	}

	acknowledgedBy = fmt.Sprintf("%s, %s", ack, acknowledgedBy)
	clone.Annotations[alertmanager.AcknowledgedByLabel] = client.LabelValue(acknowledgedBy)
	return clone
}

// AcknowledgeAlerts acknowledges multiple alerts
func AcknowledgeAlerts(alertList []*client.ExtendedAlert, acknowledgedBy string) []*client.ExtendedAlert {
	ackedAlertList := make([]*client.ExtendedAlert, len(alertList))
	for idx, alert := range alertList {
		ackedAlertList[idx] = AcknowledgeAlert(alert, acknowledgedBy)
	}
	return ackedAlertList
}

func cloneAlert(alert *client.ExtendedAlert) *client.ExtendedAlert {
	clone := *alert
	return &clone
}

// GetAlertnameFromExtendedAlert extracts the alertname label from an alert.
func GetAlertnameFromExtendedAlert(alert *client.ExtendedAlert) (string, error) {
	return findLabelValueInExtendedAlert(alert, model.AlertNameLabel)
}

// GetRegionFromExtendedAlert extracts the region label from an alert.
func GetRegionFromExtendedAlert(alert *client.ExtendedAlert) (string, error) {
	return findLabelValueInExtendedAlert(alert, alertmanager.RegionLabel)
}

// GetSeverityFromExtendedAlert extract the severity label from an alert.
func GetSeverityFromExtendedAlert(alert *client.ExtendedAlert) (string, error) {
	return findLabelValueInExtendedAlert(alert, alertmanager.SeverityLabel)
}

// MapExtendedAlertsBySeverity maps alerts to their severity for easier lookup.
func MapExtendedAlertsBySeverity(alertList []*client.ExtendedAlert) (map[string][]*client.ExtendedAlert, error) {
	var alertsFilteredBySeverity = map[string][]*client.ExtendedAlert{}
	for _, alert := range alertList {
		severity, err := GetSeverityFromExtendedAlert(alert)
		if err != nil {
			log.Printf("failed to get severity from alert %v: %v", alert.Labels, err)
			continue
		}

		severityAlerts, ok := alertsFilteredBySeverity[severity]
		if !ok {
			alertsFilteredBySeverity[severity] = []*client.ExtendedAlert{alert}
		} else {
			alertsFilteredBySeverity[severity] = append(severityAlerts, alert)
		}
	}
	return alertsFilteredBySeverity, nil
}

// CountAcknowledgedAlerts returns the number of acknowledged alerts in a list
func CountAcknowledgedAlerts(alertList []*client.ExtendedAlert) int {
	var count int
	for _, alert := range alertList {
		_, ok := alert.Labels[alertmanager.AcknowledgedByLabel]
		if ok {
			count++
		}
	}
	return count
}

// IsNoCriticalOrWarningAlerts checks whether critical or warning alerts exist
func IsNoCriticalOrWarningAlerts(alertsBySeverity map[string][]*client.ExtendedAlert) bool {
	_, criticalOK := alertsBySeverity[AlertSeverity.Critical]
	_, warningOK := alertsBySeverity[AlertSeverity.Warning]
	if !criticalOK && !warningOK {
		return true
	}
	return false
}

// ClientLabelSetToString converts a client.LabelSet to a string
func ClientLabelSetToString(labelSet client.LabelSet) string {
	var lblString string
	for k, v := range labelSet {
		lblString += fmt.Sprintf("%v=%v ", k, v)
	}
	return lblString
}

// MergeAnnotations merges annotations for mult. alerts.
func MergeAnnotations(srcAlert, trgtAlert *client.ExtendedAlert) client.LabelSet {
	trgtAnnotations := trgtAlert.Annotations
	for k, v := range srcAlert.Annotations {
		trgtAnnotations[client.LabelName(k)] = client.LabelValue(v)
	}
	return trgtAnnotations
}

// PrintableAlertSummary ...
func PrintableAlertSummary(alertsBySeverity map[string][]*client.ExtendedAlert) string {
	var region string
	for _, alertList := range alertsBySeverity {
		for _, alert := range alertList {
			r, ok := alert.Labels[alertmanager.RegionLabel]
			if ok {
				region = string(r)
			}
		}
	}

	summaryString := fmt.Sprintf("Region %s shows:\n", region)
	for severity, alerts := range alertsBySeverity {
		if len(alerts) > 0 {
			summaryString += fmt.Sprintf("• %d %s alerts. Acknowledged: %d. \n", len(alerts), severity, CountAcknowledgedAlerts(alerts))
		}
	}
	return summaryString
}

// PrintableAlertDetails ...
func PrintableAlertDetails(alertsBySeverity map[string][]*client.ExtendedAlert) string {
	detailsString := fmt.Sprintf(
		"\n| %-30s| %-20s| %-15s| %-10s| %-30s|\n", "Alertname", "Service", "Firing since", "Severity", "Acknowledged",
	)

	for _, alertList := range alertsBySeverity {
		for _, alert := range alertList {
			detailsString += fmt.Sprintf(
				"| %-30s| %-20s| %-15s| %-10s| %-30s|\n",
				alert.Labels[model.AlertNameLabel],
				alert.Labels["service"],
				util.HumanizedDurationString(time.Now().UTC().Sub(alert.StartsAt.UTC())),
				alert.Labels[alertmanager.SeverityLabel],
				alert.Labels[alertmanager.AcknowledgedByLabel],
			)
		}
	}

	return detailsString
}

func findLabelValueInExtendedAlert(alert *client.ExtendedAlert, labelName string) (string, error) {
	for ln, labelValue := range alert.Labels {
		if string(ln) == labelName {
			return string(labelValue), nil
		}
	}
	return "", fmt.Errorf("label '%s' not found in alert '%v'", labelName, alert)
}
