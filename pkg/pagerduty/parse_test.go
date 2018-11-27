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
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	summaryText         = "[#1594] \n [EU-DE-1] OpenstackLbaasApiFlapping - lbaas API flapping\n"
	summaryTextWithLink = "[#1598] \n [AP-SA-1] BaremetalIronicSensorCritical - Sensor Critical for instance node009r-bm020.cc.ap-sa-1.cloud.sap\n"
)

func TestParseAlertFromSlackMessageText(t *testing.T) {
	// mapping of input string to expected result map
	tests := map[string]map[string]string{
		summaryText: {
			"alertname": "OpenstackLbaasApiFlapping",
			"region":    "eu-de-1",
		},
		summaryTextWithLink: {
			"alertname": "BaremetalIronicSensorCritical",
			"region":    "ap-sa-1",
		},
	}

	for stimuli, expectedMap := range tests {
		actualMatchMap, err := parseRegionAndAlertnameFromPagerdutySummary(stimuli)
		assert.NoError(t, err, "there should be no error parsing the slack message text: %s", stimuli)

		assert.NotEmpty(t, actualMatchMap, "should have found the alertname in the summary text")
		assert.Equal(t, expectedMap["alertname"], actualMatchMap["alertname"], "the alertname should be equal")
		assert.Equal(t, expectedMap["region"], actualMatchMap["region"], "the region should be equal")
	}
}
