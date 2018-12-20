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

package util

import (
	"fmt"
	"log"
	"time"

	"github.com/prometheus/alertmanager/client"
	"github.com/prometheus/common/model"
	"github.com/sapcc/stargate/pkg/alertmanager"
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

// GetRegionFromAlert extracts the region label from an alert
func GetRegionFromAlert(alert *model.Alert) (string, error) {
	return findLabelValueInAlert(alert, alertmanager.RegionLabel)
}

// GetSeverityFromAlert extract the severity label from an alert
func GetSeverityFromAlert(alert *model.Alert) (string, error) {
	return findLabelValueInAlert(alert, alertmanager.SeverityLabel)
}

// GetSeverityFromExtendedAlert extract the severity label from an alert
func GetSeverityFromExtendedAlert(alert *client.ExtendedAlert) (string, error) {
	for ln, labelValue := range alert.Labels {
		if string(ln) == alertmanager.SeverityLabel {
			return string(labelValue), nil
		}
	}
	return "", fmt.Errorf("label 'severity' not found in alert '%v'", alert)
}

// MapExtendedAlertsBySeverity maps alerts to their severity for easier lookup
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
				HumanizedDurationString(time.Now().UTC().Sub(alert.StartsAt.UTC())),
				alert.Labels[alertmanager.SeverityLabel],
				alert.Labels[alertmanager.AcknowledgedByLabel],
			)
		}
	}

	return detailsString
}

// CountAcknowledgedAlerts ...
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

func findLabelValueInAlert(alert *model.Alert, labelName string) (string, error) {
	for ln, labelValue := range alert.Labels {
		if string(ln) == labelName {
			return string(labelValue), nil
		}
	}
	return "", fmt.Errorf("label '%s' not found in alert '%v'", labelName, alert)
}
