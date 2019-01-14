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

package metrics

import (
	"fmt"
	"net"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sapcc/stargate/pkg/config"
	"github.com/sapcc/stargate/pkg/log"
)

func init() {
	prometheus.MustRegister(
		HTTPRequestsTotal,
		SuccessfulOperationsTotal,
		FailedOperationsTotal,
		SnapshotSize,
		SnapshotDuration,
	)
}

// MetricNamespace ...
const MetricNamespace = "stargate"

var (
	// HTTPRequestsTotal ...
	HTTPRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "http_requests_total",
		Help:      "Count of all HTTP requests",
		Namespace: MetricNamespace,
	}, []string{"code", "method", "handler"})

	// SuccessfulOperationsTotal ...
	SuccessfulOperationsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "successful_operations_total",
		Help:      "Count of all successful operations",
		Namespace: MetricNamespace,
	}, []string{"component", "action"})

	// FailedOperationsTotal ...
	FailedOperationsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "failed_operations_total",
		Help:      "Count of all failed operations",
		Namespace: MetricNamespace,
	}, []string{"component", "action"})

	// SnapshotSize ...
	SnapshotSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      "snapshot_size",
		Help:      "Size of the snapshots",
		Namespace: MetricNamespace,
	})

	// SnapshotDuration ...
	SnapshotDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:      "snapshot_duration",
		Help:      "Duration of the snapshot",
		Namespace: MetricNamespace,
	})
)

// Serve ...
func Serve(opts config.Options, logger log.Logger) {
	host := "0.0.0.0"
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%v", host, opts.MetricPort))
	defer listener.Close()

	logger = log.NewLoggerWith(logger, "component", "metrics")
	logger.LogInfo("exposing prometheus metrics", "host", host, "port", opts.MetricPort)

	if err == nil {
		http.Serve(listener, promhttp.Handler())
	} else {
		logger.LogError("exposing prometheus metrics failed", err, "host", host, "port", opts.MetricPort)
	}
}
