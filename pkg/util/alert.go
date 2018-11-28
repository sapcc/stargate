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

	"github.com/prometheus/alertmanager/client"
	"github.com/prometheus/common/model"
	"log"
	"time"
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
	return findLabelValueInAlert(alert, "region")
}

// GetSeverityFromAlert extract the severity label from an alert
func GetSeverityFromAlert(alert *model.Alert) (string, error) {
	return findLabelValueInAlert(alert, "severity")
}

// GetSeverityFromExtendedAlert extract the severity label from an alert
func GetSeverityFromExtendedAlert(alert *client.ExtendedAlert) (string, error) {
	for ln, labelValue := range alert.Labels {
		if string(ln) == "severity" {
			return string(labelValue), nil
		}
	}
	return "", fmt.Errorf("label 'severity' not found in alert '%v'", alert)
}

// MapExtendedAlertsBySeverity ...
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

// PrintableExtendedAlertsBySeverity returns a printable version of the alerts
func PrintableExtendedAlertsBySeverity(alertsBySeverity map[string][]*client.ExtendedAlert) string {
	var msg string
	criticalAlerts, ok := alertsBySeverity[AlertSeverity.Critical]
	if ok {
		msg += fmt.Sprintf("*%v %s alerts*\n", len(criticalAlerts), AlertSeverity.Critical)
		for _, alert := range criticalAlerts {
			alertname, ok := alert.Labels[model.AlertNameLabel]
			if ok {
				firingSince := time.Now().UTC().Sub(alert.StartsAt)
				msg += fmt.Sprintf("  - %s is firing since %v. <%s|Graph>\n", alertname, HumanizedDurationString(firingSince), alert.GeneratorURL)
			}
		}
	}

	warningAlerts, ok := alertsBySeverity[AlertSeverity.Warning]
	if ok {
		msg += fmt.Sprintf("*%v %s alerts*\n", len(warningAlerts), AlertSeverity.Warning)
		for _, alert := range warningAlerts {
			alertname, ok := alert.Labels[model.AlertNameLabel]
			if ok {
				firingSince := time.Now().UTC().Sub(alert.StartsAt)
				msg += fmt.Sprintf("  - %s is firing since %v. <%s|Graph>\n", alertname, HumanizedDurationString(firingSince), alert.GeneratorURL)
			}
		}
	}

	return msg
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
