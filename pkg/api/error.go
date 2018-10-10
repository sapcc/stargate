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

package api

import "fmt"

// Error wraps an error in a json format
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"slack"`
}

// Error returns the string representation of an Error
func (err *Error) Error() string {
	return fmt.Sprintf("[%d] %s", err.Code, err.Message)
}
