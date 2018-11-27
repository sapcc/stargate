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

package slack

import (
	"fmt"
	"regexp"
	"strings"
)

const (
  // SeverityRegionRemainderRegex finds the severity and region in the alert text
  SeverityRegionRemainderRegex = `.*\*\[(?P<severity>.+?)(\s-.*)?\]\*\s\*\[(?P<region>.+?)\]\*(?P<remainder>.+?)(\>\*)?\s\-.*`

  // AlertnameRemainderRegex finds the alertname from the remainder of the alert text
  AlertnameRemainderRegex = `(.+\/#\/alerts.+\||\s)?(?P<alertname>.+)(\>\*)?`
)

func parseAlertFromSlackMessageText(text string) (map[string]string, error) {
	severityRegionRemainderRegex := regexp.MustCompile(SeverityRegionRemainderRegex)
	alertnameRemainderRegex := regexp.MustCompile(AlertnameRemainderRegex)
	matchMap := make(map[string]string)

  match := severityRegionRemainderRegex.FindStringSubmatch(text)
	for i, name := range severityRegionRemainderRegex.SubexpNames() {
		if i > 0 && i <= len(match) {
			m := match[i]
			if name == "" {
				continue
			} else if name == "severity" || name == "region" {
				m = strings.ToLower(m)
				if m == "resolved" {
					return nil, fmt.Errorf("ignoring resolved message")
				}
			} else if name == "remainder" {
				alertnameMatch := alertnameRemainderRegex.FindStringSubmatch(m)
				if alertnameMatch != nil && len(alertnameMatch) > 0 {
					matchMap["alertname"] = alertnameMatch[2]
				}
				continue
			}
			matchMap[name] = m
		}
	}

	if matchMap == nil || len(matchMap) == 0 {
		return nil, fmt.Errorf("no alert found in slack message with text '%s'", text)
	}

	return matchMap, nil
}
