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

var reactionTypes = struct {
	Acknowledge,
	SilenceUntilMonday,
	Silence1Month,
	Silence1Day string
}{
	"acknowledge",
	"silenceUntilMonday",
	"silence1Month",
	"silence1Day",
}

var commandAction = struct {
	ListAlerts string
}{
	"listAlerts",
}

const (
	// ActionName the name of the action the stargate is responding to
	ActionName = "reaction"

	// ActionType the type of the action the stargate is responding to
	ActionType = "button"

	// SilenceSuccessReactionEmoji is applied to a message after it was successfully silenced
	SilenceSuccessReactionEmoji = "silent-bell"

	// AcknowledgeReactionEmoji is applied to a message after it was successfully acknowledged
	AcknowledgeReactionEmoji = "male-firefighter"

	// SilenceDefaultComment is the default comment used for a silence
	SilenceDefaultComment = "silenced by the stargate"

	// CommandCCloud ...
	CommandCCloud = "/ccloud"
)
