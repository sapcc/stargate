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
	"time"

	alertmanager_store "github.com/prometheus/alertmanager/store"
	"github.com/sapcc/stargate/pkg/log"
	"github.com/sapcc/stargate/pkg/metrics"
	"context"
	"github.com/prometheus/common/model"
	"github.com/prometheus/alertmanager/types"
)

type AlertStore struct {
	persister *filePersister
	store     *alertmanager_store.Alerts
	logger    log.Logger
}

func NewAlertStore(gcInterval time.Duration, persister *filePersister, logger log.Logger) *AlertStore {
	logger = log.NewLoggerWith(logger, "component", "AlertStore")

	// load existing store or create a new
	store, err := persister.Load()
	if err != nil {
		logger.LogInfo("cannot load alert store. creating new one", "err", err)
		store = alertmanager_store.NewAlerts(gcInterval)
	}

	a := &AlertStore{
		store:     store,
		persister: persister,
		logger:    logger,
	}
	return a
}

func (a *AlertStore) Run(ctx context.Context) {
	a.store.Run(ctx)
}

func (a *AlertStore) Get(fp model.Fingerprint) (*types.Alert, error) {
	return a.store.Get(fp)
}

func (a *AlertStore) GetFromFingerPrintString(fpString string) (*types.Alert, error) {
	fp, err := model.FingerprintFromString(fpString)
	if err != nil {
		return nil, err
	}
	return a.store.Get(fp)
}

func (a *AlertStore) Set(alert *types.Alert) error {
	return a.store.Set(alert)
}

func (a *AlertStore) Count() int {
	return a.store.Count()
}

func (a *AlertStore) List() []*types.Alert {
	a.store.Lock()
	defer a.store.Unlock()
	
	alertList := make([]*types.Alert, 0)
	for alert := range a.store.List() {
		alertList = append(alertList, alert)
	}
	return alertList
}

func (a *AlertStore) Delete(fp model.Fingerprint) error {
	return a.store.Delete(fp)
}

// Snapshot creates a snapshot of the current store
func (a *AlertStore) Snapshot() error {
	var snapshotSize int64
	start := time.Now()
	a.store.Lock()
	defer a.store.Unlock()
	
	defer func() {
		snapShotDuration := time.Since(start).Seconds()
		a.logger.LogInfo("persisted alert snapshot", "size", snapshotSize, "duration (s)", snapShotDuration)
		metrics.SnapshotDuration.Observe(snapShotDuration)
		metrics.SnapshotSize.Set(float64(snapshotSize))
	}()
	
	snapshotSize, err := a.persister.Store(a.store)
	if err != nil {
		return err
	}
	return nil
}
