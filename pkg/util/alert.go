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

	"github.com/prometheus/common/model"
)

// GetRegionFromAlert extracts the region label from an alert
func GetRegionFromAlert(alert *model.Alert) (string, error) {
	for labelName, labelValue := range alert.Labels {
		if labelName == "region" {
			return string(labelValue), nil
		}
	}
	return "", fmt.Errorf("no region label found in alert '%v'", alert)
}
