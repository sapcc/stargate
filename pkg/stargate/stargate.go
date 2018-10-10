package stargate

import (
	"log"
	"net/http"

	"github.com/sapcc/stargate/pkg/alertmanager"
	"github.com/sapcc/stargate/pkg/api"
	"github.com/sapcc/stargate/pkg/slack"
  "github.com/sapcc/stargate/pkg/config"
)

// Stargate ...
type Stargate struct {
	v1API *api.API

	alertmanagerClient alertmanager.Alertmanager
	slack              slack.Receiver

	Config config.Config
}

// NewStargate creates a new stargate
func NewStargate(cfg config.Config) *Stargate {

	if cfg.ConfigFilePath == "" {
		log.Println("path to configuration file not provided")
	} else {
		c, err := config.NewConfig(cfg.ConfigFilePath)
		if err != nil {
			log.Fatal(err)
		}
		cfg = c
	}
	sg := &Stargate{
		Config: cfg,
		slack:  slack.New(cfg),
	}

	v1API := api.NewV1API(cfg)
	v1API.PathPrefix("/v1")
	v1API.AddRoutes(
		[]api.Route{
			{
				http.MethodPost,
				"/slack",
				sg.slack.HandleMessage,
			},
		},
	)
	sg.v1API = v1API
	return sg
}

// Run starts the stargate
func (s *Stargate) Run() {
	err := s.v1API.Serve()
	if err != nil {
		log.Fatal(err)
	}
}