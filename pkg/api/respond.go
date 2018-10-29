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

import (
	"encoding/json"
	"net/http"
)

// RespondWithOK responds with 200
func RespondWithOK(w http.ResponseWriter) {
  w.WriteHeader(http.StatusOK)
}

// RespondWithUnauthorized responds with an appropriate json error
func RespondWithUnauthorized(w http.ResponseWriter) {
  json.NewEncoder(w).Encode(
    Error{
      Code: http.StatusUnauthorized,
      Message: "Unauthorized",
    },
  )
}

// RespondWithError responds with a JSON error
func RespondWithError(code int, message string, w http.ResponseWriter) {
	json.NewEncoder(w).Encode(
		Error{
			Code:    code,
			Message: message,
		},
	)
}
