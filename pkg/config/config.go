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

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/pkg/errors"
	"github.com/sapcc/stargate/pkg/log"
	"gopkg.in/yaml.v2"
)

// Config ...
type Config struct {
	AlertManager alertmanagerConfig `yaml:"alertmanager"`
	Slack        slackConfig        `yaml:"slack"`
	Pagerduty    pagerdutyConfig    `yaml:"pagerduty"`

	ListenPort  int
	ExternalURL string

	ConfigFilePath string
	SecretFilePath string
}

type alertmanagerConfig struct {
	URL string `yaml:"url"`
}

type slackConfig struct {
	AuthorizedGroups []string `yaml:"authorized_groups"`

	// AccessToken to authenticate the stargate to messenger
	AccessToken string `yaml:"access_token"`

	// BotUserAccessToken is the access token used by the bot
	BotUserAccessToken string `yaml:"bot_user_access_token"`

	// SigningSecret to verify slack messages
	SigningSecret string `yaml:"signing_secret"`

	// VerificationToken to verify slack messages
	VerificationToken string `yaml:"verification_token"`

	// UserName for slack messages
	UserName string `yaml:"user_name"`

	// UserIcon for slack messages
	UserIcon string `yaml:"user_icon"`

	// Command to trigger actions
	Command string `yaml:"command"`

	// RecheckInterval for user group memberships
	RecheckInterval time.Duration `yaml:"recheck_interval"`

	// IsDisableRTM allows disabeling the slack RTM (real time messaging)
	IsDisableRTM bool `yaml:"-"`
}

type pagerdutyConfig struct {
	// AuthToken used to authenticate with pagerduty
	AuthToken string `yaml:"auth_token"`
}

// NewConfig reads the configuration from the given filePath
func NewConfig(opts Options, logger log.Logger) (cfg Config, err error) {
	if opts.ConfigFilePath == "" {
		return cfg, errors.New("path to configuration file not provided")
	}

	cfgBytes, err := ioutil.ReadFile(opts.ConfigFilePath)
	if err != nil {
		return cfg, fmt.Errorf("read configuration file: %s", err.Error())
	}
	err = yaml.Unmarshal(cfgBytes, &cfg)
	if err != nil {
		return cfg, fmt.Errorf("parse configuration: %s", err.Error())
	}

	if opts.ExternalURL != "" {
		cfg.ExternalURL = opts.ExternalURL
	}
	if opts.ListenPort != 0 {
		cfg.ListenPort = opts.ListenPort
	}
	cfg.Slack.IsDisableRTM = opts.IsDisableSlackRTM

	if err := cfg.Slack.validate(); err != nil {
		logger.LogFatal("invalid slack configuration", "err", err)
	}

	if err := cfg.AlertManager.validate(); err != nil {
		logger.LogFatal("invalid alertmanager configuration", "err", err)
	}

	return cfg, nil
}

func (s *slackConfig) validate() error {
	if s.SigningSecret == "" && s.VerificationToken == "" {
		return errors.New("incomplete messenger configuration: either messenger `signing_secret` or `verification_token` needs to be provided so messenger messenger can be verified")
	}

	if s.AccessToken == "" {
		return errors.New("incomplete messenger configuration: missing messenger `access_token`")
	}

	if s.UserName == "" {
		s.UserName = "Stargate"
	}

	if s.Command == "" {
		s.Command = "/stargate"
	}

	if s.RecheckInterval == 0 {
		s.RecheckInterval = 1 * time.Hour
	}

	return nil
}

func (a *alertmanagerConfig) validate() error {
	if a.URL == "" {
		return errors.New("missing `alertmanager.url` in config")
	}

	return nil
}

// GetValidationToken returns either the signingSecret or verificationToken in order to validate slack messenger
func (s *slackConfig) GetValidationToken() string {
	if s.SigningSecret != "" {
		return s.SigningSecret
	}
	return s.VerificationToken
}
