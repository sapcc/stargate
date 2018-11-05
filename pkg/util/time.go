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
	"strings"
	"time"

	"github.com/dustin/go-humanize"
)

// HumanizedDurationString returns a humanized string of a duration
// examples: 8h0m0s => 8 hours, 168h0m0s => 1 week
func HumanizedDurationString(duration time.Duration) string {
	now := time.Now().UTC()
	humanizedDurationString := humanize.RelTime(
		now,
		now.Add(duration),
		"",
		"",
	)
	return strings.TrimSpace(humanizedDurationString)
}

// TimeUntilNextMonday returns the number of days until the next monday
func TimeUntilNextMonday(t0 time.Time) int {
	daysToAdd := (int(time.Monday) - int(t0.Weekday()) + 7) % 7;
	if daysToAdd == 0 {
		daysToAdd = 7
	}
	return daysToAdd
}

// DaysToHours returns the given number of days in hours
func DaysToHours(days int) time.Duration {
	return time.Duration(days) * 24 * time.Hour
}