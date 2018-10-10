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

	"gopkg.in/yaml.v1"
	"github.com/apex/log"
)

// Config ...
type Config struct {
	AlertManager alertmanagerConfig

	ListenPort  uint   `yaml:",inline"`
	ExternalURL string `yaml:",inline"`

	ConfigFilePath string `yaml:",inline"`
}

type alertmanagerConfig struct {
	URL string `yaml:"alertmanager_url"`
}

// NewConfig reads the configuration from the given filePath
func NewConfig(filePath string) (cfg Config, err error) {
	if filePath == "" {
		log.Info("path to configuration file not provided")
		return cfg, nil
	}

	cfgBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return cfg, fmt.Errorf("read configuration file: %s", err.Error())
	}
	err = yaml.Unmarshal(cfgBytes, &cfg)
	if err != nil {
		return cfg, fmt.Errorf("parse configuration: %s", err.Error())
	}

	return cfg, nil
}
