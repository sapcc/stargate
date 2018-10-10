package main

import (
	"flag"
	"log"
	"os"

	"github.com/sapcc/stargate/pkg/config"
	"github.com/sapcc/stargate/pkg/stargate"
	"github.com/spf13/pflag"
	"os/signal"
	"syscall"
	"sync"
)

var cfg config.Config

func init() {
	pflag.StringVar(&cfg.AlertManager.URL, "alertmanager-url", "", "URL of the Prometheus Alertmanager")
	pflag.UintVar(&cfg.ListenPort, "port", 8080, "API port")
	pflag.StringVar(&cfg.ExternalURL, "external-url", "", "External URL")
	pflag.StringVar(&cfg.ConfigFilePath, "config-file", "", "Path to config file")
}

func main() {
	log.SetOutput(os.Stdout)

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	sigs := make(chan os.Signal, 1)
	stop := make(chan struct{})
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	wg := &sync.WaitGroup{}

	go stargate.NewStargate(cfg).Run()

	<-sigs // Wait for signals (this hangs until a signal arrives)
	log.Println("Shutting down...")

	close(stop) // Tell goroutines to stop themselves
	wg.Wait()   // Wait for all to be stopped
}
