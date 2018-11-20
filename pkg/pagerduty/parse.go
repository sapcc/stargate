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

package pagerduty

import (
  "regexp"
  "strings"
  "fmt"
)

const RegionAlertnameRegex = `.*\s\[(?P<region>.+?)\]\s(?P<alertname>.+?)\s\-.*`

func parseRegionAndAlertnameFromPagerdutySummary(summary string) (map[string]string, error) {
  regionAlertnameRegex := regexp.MustCompile(RegionAlertnameRegex)
  matchMap := make(map[string]string)

  match := regionAlertnameRegex.FindStringSubmatch(summary)
  for i, name := range regionAlertnameRegex.SubexpNames() {
    if i > 0 && i <= len(match) {
      m := match[i]
      if name == "" {
        continue
      } else if name == "region" {
        m = strings.ToLower(m)
      }
      matchMap[name] = m
    }
  }

  if len(matchMap) == 0 {
    return nil, fmt.Errorf("pagerduty incident summary doesn not contain alertname and/or region: '%s'", summary)
  }

  return matchMap, nil
}
