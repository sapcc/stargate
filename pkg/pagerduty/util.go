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

package pagerduty

import (
	"strings"

	"github.com/sapcc/go-pagerduty"
)

func acknowledgementsToString(acknowledgements []pagerduty.Acknowledgement) string {
	var ackSlice []string
	for _, ack := range acknowledgements {
		ackSlice = append(ackSlice, ack.Acknowledger.ID)
	}
	return strings.Join(ackSlice, ", ")
}

func assignmentsToString(assignments []pagerduty.Assignment) string {
	var assSlice []string
	for _, ass := range assignments {
		assSlice = append(assSlice, ass.Assignee.ID)
	}
	return strings.Join(assSlice, ", ")
}
