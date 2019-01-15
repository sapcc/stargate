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

package config

import "time"

// Options passed via cmd line
type Options struct {
	ListenPort          int
	MetricPort          int
	IsDebug             bool
	IsDisableSlackRTM   bool
	ExternalURL         string
	ConfigFilePath      string
	PersistenceFilePath string
	RecheckInterval     time.Duration
}
