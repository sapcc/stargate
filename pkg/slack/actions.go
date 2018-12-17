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

import "strings"

// Action struct for available actions that can be triggered
var Action = struct {
	ShowAlerts string
}{
	"showAlerts",
}

// commandActions mapping of action to keywords (commands)
var commandActions = map[string][]string{
	Action.ShowAlerts: {"show", "alerts"},
}

func textContainsAllKeyWords(text string, keywords []string) bool {
	for _, k := range keywords {
		if !strings.Contains(text, k) {
			return false
		}
	}
	return true
}
