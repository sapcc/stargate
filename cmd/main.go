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
	"log"
	"os"

	"os/signal"
	"sync"
	"syscall"

	"github.com/sapcc/stargate/pkg/config"
	"github.com/sapcc/stargate/pkg/stargate"
	"github.com/spf13/pflag"
)

var opts config.Options

func init() {
	pflag.StringVar(&opts.AlertmanagerURL, "alertmanager-url", "", "URL of the Prometheus Alertmanager")
	pflag.StringVar(&opts.ExternalURL, "external-url", "", "External URL")
	pflag.IntVar(&opts.ListenPort, "port", 8080, "API port")
	pflag.StringVar(&opts.ConfigFilePath, "config-file", "/etc/stargate/config/stargate.yaml", "Path to the file containing the config")
	pflag.BoolVar(&opts.IsDebug, "debug", false, "Enable debug configuration and log level")
}

func main() {
	log.SetOutput(os.Stdout)

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	sigs := make(chan os.Signal, 1)
	stop := make(chan struct{})
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	wg := &sync.WaitGroup{}

	go stargate.NewStargate(opts).Run()

	<-sigs // Wait for signals (this hangs until a signal arrives)
	log.Println("Shutting down...")

	close(stop) // Tell goroutines to stop themselves
	wg.Wait()   // Wait for all to be stopped
}
