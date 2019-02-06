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

package store

import (
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/alertmanager/client"
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
	err := rmPersistedAlertStoreIfExists()
	require.NoError(t, err, "ensuring the alertstore file is removed must not raise an error")

	persister, err := newPersister()
	require.NoError(t, err, "creating a persister must not raise an error")

	alertStore, err := newAlertStore()
	require.NoError(t, err, "creating an alert store must not raise an error")
	require.NotZero(t, alertStore.Count(), "the alert store must not be empty for this test")

	size, err := persister.Store(alertStore.s)
	assert.NoError(t, err, "persisting an alert store should not raise an error")
	assert.NotZero(t, size, "the size of the persisted alert store should not be 0")
}

func TestLoadAlertStore(t *testing.T) {
	persister, err := newPersister()
	require.NoError(t, err, "creating a persister must not raise an error")

	alertStoreMap, err := persister.Load()
	assert.NoError(t, err, "loading an alert store should not raise an error")
	assert.NotEmpty(t, alertStoreMap, "the alert store must not be empty for this test")
	assert.Len(t, alertStoreMap, 2, "the loaded alert store should have a length of 2")
	assertMapContainsKey(t, alertStoreMap, "05281b4f8947b35c", "the loaded store should contain an alert with fingerprint '05281b4f8947b35c'")
	assertMapContainsKey(t, alertStoreMap, "05281b4f8947b35d", "the loaded store should contain an alert with fingerprint '05281b4f8947b35d'")
}

func newAlertStore() (*AlertStore, error) {
	alertList := []*client.ExtendedAlert{
		{
			Fingerprint: "05281b4f8947b35c",
			Alert: client.Alert{
				Labels: client.LabelSet{
					model.AlertNameLabel: "quarkNase",
					"Quark":              "Nase",
				},
				Annotations: client.LabelSet{
					"acknowledgedBy": "user1",
				},
				StartsAt:     time.Now().UTC(),
				EndsAt:       time.Now().UTC().Add(1 * time.Hour),
				GeneratorURL: "generatorURL",
			},
		},
		{
			Fingerprint: "05281b4f8947b35d",
			Alert: client.Alert{
				Labels: client.LabelSet{
					model.AlertNameLabel: "Boogieman",
					"Boogie":             "Man",
				},
				Annotations: client.LabelSet{
					"acknowledgedBy": "user2",
				},
				StartsAt:     time.Now().UTC(),
				EndsAt:       time.Now().UTC().Add(1 * time.Hour),
				GeneratorURL: "generatorURL",
			},
		},
	}

	store := &AlertStore{
		s:               map[model.Fingerprint]*client.ExtendedAlert{},
		logger:          log.NewLogger(true),
		mtx:             sync.RWMutex{},
		recheckInterval: 5 * time.Minute,
	}

	for _, alert := range alertList {
		if err := store.Set(alert); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func newPersister() (*FilePersister, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Ensure fixtures dir exists.
	if err := os.Mkdir(path.Join(pwd, PathFixtures), 777); err != nil {
		if !os.IsExist(err) {
			return nil, errors.Wrapf(err, "error creating directory '%s'", PathFixtures)
		}
	}

	return NewFilePersister(path.Join(pwd, PathFixtures, FileNamePersistetAlertStore), log.NewLogger(true))
}

func rmPersistedAlertStoreIfExists() error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	err = os.Remove(path.Join(pwd, PathFixtures, FileNamePersistetAlertStore))
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}

func assertMapContainsKey(t *testing.T, m map[model.Fingerprint]*client.ExtendedAlert, containsFingerprintString string, errorText string) {
	for fp := range m {
		if fp.String() == containsFingerprintString {
			return
		}
	}
	t.Errorf(errorText)
}
