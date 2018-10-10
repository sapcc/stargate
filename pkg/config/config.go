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

package config

import (
	"fmt"
	"io/ioutil"

	"github.com/apex/log"
	"gopkg.in/yaml.v2"
)

// Config ...
type Config struct {
	AlertManager alertmanagerConfig `yaml:"alertmanager"`
	SlackConfig  slackConfig        `yaml:"slack"`

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

	// the AccessToken to authenticate the stargate to messenger
	AccessToken string `yaml:"access_token"`

	// signing token to verify messenger messenger
	SigningSecret string `yaml:"signing_secret"`

	// verification token to verify messenger messenger
	VerificationToken string `yaml:"verification_token"`
}

// NewConfig reads the configuration from the given filePath
func NewConfig(opts Options) (cfg Config, err error) {
	if opts.ConfigFilePath == "" {
		log.Info("path to configuration file not provided")
		return cfg, nil
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
	if opts.AlertmanagerURL != "" {
		cfg.AlertManager.URL = opts.AlertmanagerURL
	}

	cfg.SlackConfig.validate()

	return cfg, nil
}

func (s slackConfig) validate() {
	if s.SigningSecret == "" && s.VerificationToken == "" {
		log.Fatal("incomplete messenger configuration: either messenger `signing_secret` or `verification_token` needs to be provided so messenger messenger can be verified")
	}

	if s.AccessToken == "" {
		log.Fatal("incomplete messenger configuration: missing messenger `access_token`")
	}
}

// GetValidationToken returns either the signingSecret or verificationToken in order to validate slack messenger
func (s *slackConfig) GetValidationToken() string {
	if s.SigningSecret != "" {
		return s.SigningSecret
	}
	return s.VerificationToken
}
