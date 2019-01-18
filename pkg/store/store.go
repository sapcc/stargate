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
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/alertmanager/client"
	"github.com/prometheus/common/model"
	alert_util "github.com/sapcc/stargate/pkg/alert"
	"github.com/sapcc/stargate/pkg/alertmanager"
	"github.com/sapcc/stargate/pkg/config"
	"github.com/sapcc/stargate/pkg/log"
	"github.com/sapcc/stargate/pkg/metrics"
)

var (
	// ErrNotFound is exactly that.
	ErrNotFound = errors.New("alert not found")
)

// AlertStore ...
type AlertStore struct {
	alertmanagerClient *alertmanager.Client
	recheckInterval    time.Duration
	mtx                sync.RWMutex
	persister          *FilePersister
	logger             log.Logger

	// internal store with modified alert
	s map[model.Fingerprint]*client.ExtendedAlert
}

// NewAlertStore creates a new AlertStore.
func NewAlertStore(cfg config.Config, recheckInterval time.Duration, persister *FilePersister, logger log.Logger) *AlertStore {
	logger = log.NewLoggerWith(logger, "component", "alertstore")

	// load existing store or create a new
	store, err := persister.Load()
	if err != nil {
		logger.LogDebug("cannot load alert store. creating new one", "err", err)
		store = make(map[model.Fingerprint]*client.ExtendedAlert)
	}

	return &AlertStore{
		alertmanagerClient: alertmanager.New(cfg, logger),
		recheckInterval:    recheckInterval,
		mtx:                sync.RWMutex{},
		persister:          persister,
		logger:             logger,
		s:                  store,
	}
}

// Run runs the AlertStore.
func (a *AlertStore) Run(wg *sync.WaitGroup, stopCh <-chan struct{}) {
	wg.Add(1)
	defer wg.Done()

	a.logger.LogInfo("running alert store")
	ticker := time.NewTicker(a.recheckInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := a.garbageCollect(); err != nil {
					a.logger.LogError("garbage collection failed", err)
				}
			case <-stopCh:
				ticker.Stop()
				return
			}
		}
	}()
	<-stopCh
}

// Get returns an alert for a given Fingerprint or an error.
func (a *AlertStore) Get(fp model.Fingerprint) (*client.ExtendedAlert, error) {
	a.mtx.RLock()
	defer a.mtx.RUnlock()

	alert, ok := a.s[fp]
	if !ok {
		return nil, ErrNotFound
	}
	a.logger.LogDebug("getting alert from store", "fingerprint", fp.String())
	return alert, nil
}

// GetFromFingerPrintString returns an alert for a given Fingerprint or an error.
func (a *AlertStore) GetFromFingerPrintString(fpString string) (*client.ExtendedAlert, error) {
	fp, err := model.FingerprintFromString(fpString)
	if err != nil {
		return nil, err
	}
	return a.Get(fp)
}

// Set adds an alert to the AlertStore.
func (a *AlertStore) Set(extendedAlert *client.ExtendedAlert) error {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	fp, err := model.FingerprintFromString(extendedAlert.Fingerprint)
	if err != nil {
		return err
	}
	a.s[fp] = extendedAlert
	a.logger.LogDebug("adding alert to store", "fingerprint", fp.String())
	return nil
}

// AcknowledgeAndSetMultiple acknowledges and adds multiple alerts to the AlertStore.
// If alert already present in AlertStore, additional acknowledgers will be appended.
func (a *AlertStore) AcknowledgeAndSetMultiple(extendedAlertList []*client.ExtendedAlert, acknowledgedBy string) error {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	ackedAlertList := alert_util.AcknowledgeAlerts(extendedAlertList, acknowledgedBy)
	for _, ackedAlert := range ackedAlertList {
		fp, err := model.FingerprintFromString(ackedAlert.Fingerprint)
		if err != nil {
			a.logger.LogError("failed to create fingerprint for alert. ignoring", err)
			continue
		}

		foundAlert, ok := a.s[fp]
		if !ok {
			a.s[fp] = ackedAlert
			a.logger.LogDebug("adding alert to store", "fingerprint", fp.String())
			continue
		}

		// The alert was found in the store, which means it was acknowledged previously.
		// So we need to append the new acknowledger to the list.
		a.s[fp] = alert_util.AcknowledgeAlert(foundAlert, acknowledgedBy)
	}
	return nil
}

// UpdateAlertEndsAt updates the EndsAt field of an alert in the AlertStore.
func (a *AlertStore) UpdateAlertEndsAt(extendedAlert *client.ExtendedAlert) error {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	fp, err := model.FingerprintFromString(extendedAlert.Fingerprint)
	if err != nil {
		return err
	}

	_, ok := a.s[fp]
	if !ok {
		return ErrNotFound
	}

	a.s[fp].EndsAt = extendedAlert.EndsAt
	return nil
}

// Count returns the number of items in the AlertStore.
func (a *AlertStore) Count() int {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	return len(a.s)
}

// List returns a list of alerts in the AlertStore.
func (a *AlertStore) List() []*client.ExtendedAlert {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	alertList := make([]*client.ExtendedAlert, len(a.s))
	for _, alert := range a.s {
		alertList = append(alertList, alert)
	}

	return alertList
}

// Delete removes an item from the AlertStore.
func (a *AlertStore) Delete(fp model.Fingerprint) error {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	delete(a.s, fp)
	a.logger.LogDebug("deleting alert from store", "fingerprint", fp.String())
	return nil
}

// Snapshot creates a snapshot of the current store
func (a *AlertStore) Snapshot() error {
	var snapshotSize int64
	start := time.Now()
	a.mtx.Lock()
	defer a.mtx.Unlock()

	defer func() {
		snapShotDuration := time.Since(start).Seconds()
		a.logger.LogInfo("persisted alert snapshot", "size", snapshotSize, "duration (s)", snapShotDuration)
		metrics.SnapshotDuration.Observe(snapShotDuration)
		metrics.SnapshotSize.Set(float64(snapshotSize))
	}()

	snapshotSize, err := a.persister.Store(a.s)
	if err != nil {
		return err
	}
	return nil
}

// garbageCollect cleans the AlertStore.
// Alerts which are not present in the Alertmanager, or Alerts which expired will be removed.
func (a *AlertStore) garbageCollect() error {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	filter := alertmanager.NewDefaultFilter()
	filter.IsSilenced = true
	currentAlertList, err := a.alertmanagerClient.ListAlerts(filter)
	if err != nil {
		return errors.Wrapf(err, "failed to list alerts from alertmanager")
	}

	// Create a map for easier lookup of alerts by Fingerprint.
	currentAlertMap := make(map[model.Fingerprint]*client.ExtendedAlert)
	for _, alert := range currentAlertList {
		fp, err := model.FingerprintFromString(alert.Fingerprint)
		if err != nil {
			a.logger.LogError("failed to create fingerprint for alert. ignoring", err)
			continue
		}
		currentAlertMap[fp] = alert
	}

	for fp := range a.s {
		al, ok := currentAlertMap[fp]
		if !ok {
			delete(a.s, fp)
			a.logger.LogDebug("alert can no longer be found in alertmanager. deleting from store", "fingerprint", fp.String())
			continue
		} else if al.EndsAt.UTC().After(time.Now().UTC()) {
			delete(a.s, fp)
			a.logger.LogDebug("alert is expired. deleting alert from store", "fingerprint", fp.String())
			continue
		}
		// Update the EndsAt of the alert in the AlertStore with the one found in the Alertmanager.
		a.s[fp].EndsAt = currentAlertMap[fp].EndsAt
	}
	return nil
}

// IsErrNotFound checks whether the error is an ErrNotFound.
func IsErrNotFound(err error) bool {
	if err != nil {
		return err.Error() == ErrNotFound.Error()
	}
	return false
}
