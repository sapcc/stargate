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

package store

import (
	"os"
	"path"
	"testing"
	"time"

	alertmanager_store "github.com/prometheus/alertmanager/store"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/sapcc/stargate/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	PathFixtures                = "fixtures"
	FileNamePersistetAlertStore = "alerts.dump"
)

func TestPersistAlertStore(t *testing.T) {
	persister, err := newPersister()
	require.NoError(t, err, "creating a persister must not raise an error")

	alertStore, err := newAlertStore()
	require.NoError(t, err, "creating an alert store must not raise an error")
	require.NotZero(t, alertStore.Count(), "the alert store must not be empty for this test")

	size, err := persister.Store(alertStore)
	assert.NoError(t, err, "persisting an alert store should not raise an error")
	assert.NotZero(t, size, "the size of the persisted alert store should not be 0")
}

func TestLoadAlertStore(t *testing.T) {
	persister, err := newPersister()
	require.NoError(t, err, "creating a persister must not raise an error")

	alertStore, err := persister.Load()
	assert.NoError(t, err, "loading an alert store should not raise an error")
	assert.NotZero(t, alertStore.Count(), "the alert store must not be empty for this test")

}

func newAlertStore() (*alertmanager_store.Alerts, error) {
	alertList := []*types.Alert{
		{
			Alert: model.Alert{
				Labels: model.LabelSet{
					model.AlertNameLabel: "quarkNase",
					"Quark":              "Nase",
				},
				Annotations: model.LabelSet{
					"acknowledgedBy": "user1",
				},
				StartsAt:     time.Now().UTC(),
				EndsAt:       time.Now().UTC().Add(1 * time.Hour),
				GeneratorURL: "generatorURL",
			},
		},
		{
			Alert: model.Alert{
				Labels: model.LabelSet{
					model.AlertNameLabel: "Boogieman",
					"Boogie":             "Man",
				},
				Annotations: model.LabelSet{
					"acknowledgedBy": "user2",
				},
				StartsAt:     time.Now().UTC(),
				EndsAt:       time.Now().UTC().Add(1 * time.Hour),
				GeneratorURL: "generatorURL",
			},
		},
	}

	store := alertmanager_store.NewAlerts(5 * time.Minute)
	for _, alert := range alertList {
		if err := store.Set(alert); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func newPersister() (*filePersister, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	return NewFilePersister(
		path.Join(pwd, PathFixtures, FileNamePersistetAlertStore),
		log.NewLogger(),
	)
}
