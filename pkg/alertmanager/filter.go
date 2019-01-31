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

package alertmanager

import (
	"fmt"
	"github.com/prometheus/alertmanager/client"
	"net/http"
	"strings"
)

// Filter is used to filter alerts.
type Filter struct {
	IsSilenced,
	isOnlySilence,
	IsInhibited,
	IsActive,
	IsUnprocessed bool
	Receiver  string
	AddFilter string
}

// NewDefaultFilter returns a new default filter.
func NewDefaultFilter() *Filter {
	return &Filter{
		IsSilenced:    false,
		isOnlySilence: false,
		IsInhibited:   true,
		IsActive:      true,
		IsUnprocessed: false,
		Receiver:      "",
		AddFilter:     "",
	}
}

// NewFilterFromRequest returns a new filter from an request.
func NewFilterFromRequest(r *http.Request) *Filter {
	f := NewDefaultFilter()

	query := r.URL.Query()
	for k, v := range query {
		switch k {
		case "silenced":
			f.IsSilenced, f.isOnlySilence = evalSilencedFilter(v)
		case "inhibited":
			f.IsInhibited = toBool(v)
		case "active":
			f.IsActive = toBool(v)
		case "unprocessed":
			f.IsUnprocessed = toBool(v)
		case "receiver":
			f.Receiver = strings.Join(v, ",")
		case "filter":
			for _, itm := range v {
				f.AddFilter += strings.TrimPrefix(itm, "filter=")
			}
		default:
			continue
		}
	}
	return f
}

// WithAdditionalFilter adds an additional filter
func (f *Filter) WithAdditionalFilter(addFilter map[string]string) {
	for k, v := range addFilter {
		f.AddFilter += fmt.Sprintf("%s=%s", k, v)
	}
}

// WithAlertLabelsFilter adds a filter based on alert labels
func (f *Filter) WithAlertLabelsFilter(lblset client.LabelSet) {
	filterList := make([]string, 0)
	for k, v := range lblset {
		filterList = append(filterList, fmt.Sprintf("%s=\"%s\"", string(k), string(v)))
	}
	// add trailing "," if not already if necessary.
	if f.AddFilter != "" && !strings.HasSuffix(f.AddFilter, ",") {
		f.AddFilter += ","
	}
	f.AddFilter += strings.Join(filterList, ",")
}

func (f *Filter) toString() string {
	return f.AddFilter
}

func toBool(s []string) bool {
	for _, v := range s {
		if v == "false" {
			return false
		}
	}
	return true
}

// returns isSilenced, isOnlySilenced
func evalSilencedFilter(s []string) (bool, bool) {
	for _, v := range s {
		switch v {
		case "false":
			return false, false
		case "true":
			return true, false
		case "only":
			return true, true
		}
	}
	return false, false
}
