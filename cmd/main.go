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

package main

import (
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/sapcc/stargate/pkg/config"
	"github.com/sapcc/stargate/pkg/log"
	"github.com/sapcc/stargate/pkg/metrics"
	"github.com/sapcc/stargate/pkg/stargate"
	"github.com/spf13/pflag"
)

var opts config.Options

func init() {
	pflag.StringVar(&opts.ExternalURL, "external-url", "", "External URL")
	pflag.IntVar(&opts.ListenPort, "port", 8080, "API port")
	pflag.IntVar(&opts.MetricPort, "metric-port", 9090, "Metric port")
	pflag.StringVar(&opts.ConfigFilePath, "config-file", "/etc/stargate/config/stargate.yaml", "Path to the file containing the config")
	pflag.BoolVar(&opts.IsDebug, "debug", false, "Enable debug configuration and log level")
	pflag.BoolVar(&opts.IsDisableSlackRTM, "disable-slack-rtm", false, "Disable Slack RTM (the bot)")
}

func main() {
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	sigs := make(chan os.Signal, 1)
	stop := make(chan struct{})
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	logger := log.NewLogger()

	wg := &sync.WaitGroup{}

	go stargate.New(opts).Run(wg, stop)
	go metrics.Serve(opts, logger)

	<-sigs // Wait for signals (this hangs until a signal arrives)
	logger.LogInfo("shutting down...")

	close(stop) // Tell goroutines to stop themselves
	wg.Wait()   // Wait for all to be stopped
}
