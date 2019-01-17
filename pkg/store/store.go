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
	"github.com/sapcc/stargate/pkg/alert"
	"github.com/sapcc/stargate/pkg/alertmanager"
	"github.com/sapcc/stargate/pkg/config"
	"github.com/sapcc/stargate/pkg/log"
	"github.com/sapcc/stargate/pkg/metrics"
	"github.com/sapcc/stargate/pkg/util"
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
	// regularly cached alerts from alertmanager
	alertCache map[model.Fingerprint]*client.ExtendedAlert
}

// NewAlertStore creates a new AlertStore.
func NewAlertStore(cfg config.Config, recheckInterval time.Duration, persister *FilePersister, logger log.Logger) *AlertStore {
	logger = log.NewLoggerWith(logger, "component", "alertStore")

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
		alertCache:         make(map[model.Fingerprint]*client.ExtendedAlert),
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
				err := a.syncWithAlertmanager()
				if err != nil {
					a.logger.LogError("sync with alertmanager failed", err)
				} else {
					a.garbageCollect()
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
func (a *AlertStore) Set(alert *client.ExtendedAlert) error {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	fp, err := model.FingerprintFromString(alert.Fingerprint)
	if err != nil {
		return err
	}
	a.s[fp] = alert
	a.logger.LogDebug("adding alert to store", "fingerprint", fp.String())
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

// AcknowledgeAlert acknowledges an alert and adds it to the AlertStore.
func (a *AlertStore) AcknowledgeAlert(al *client.ExtendedAlert, acknowledgedBy string) error {
	// find alert in cache. assuming it is resolved if not found.
	var err error
	extendedAlertList, err := a.findAlertsInCache(al.Labels)
	if err != nil {
		return errors.Wrapf(err, "could not find alert in cache with labels '%s'", alert.ClientLabelSetToString(al.Labels))
	}

	ackedAlertList := alert.AcknowledgeAlerts(extendedAlertList, acknowledgedBy)
	for _, al := range ackedAlertList {
		if err := a.Set(al); err != nil {
			a.logger.LogError("failed to add alert to store", err, "fingerprint", al.Fingerprint)
		}
	}
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

func (a *AlertStore) garbageCollect() {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	for fp, al := range a.s {
		endsAt, isResolved := a.isAlertStillFiring(al)
		alertname, err := alert.GetAlertnameFromExtendedAlert(al)
		if err != nil {
			alertname = fp.String()
		}
		if isResolved {
			delete(a.s, fp)
			a.logger.LogDebug("garbage collecting expired alert", "alert", alertname)
		} else {
			a.s[fp].EndsAt = endsAt
			a.logger.LogDebug("updating alert expiry", "alert", alertname)
		}
	}
}

func (a *AlertStore) isAlertStillFiring(alert *client.ExtendedAlert) (time.Time, bool) {
	isResolved := false
	if time.Now().UTC().After(alert.EndsAt.UTC()) {
		isResolved = true
	}

	fp, err := model.FingerprintFromString(alert.Fingerprint)
	if err != nil {
		a.logger.LogError("failed to create fingerprint for alert", err)
		return alert.EndsAt.UTC(), false
	}

	cachedAlert, ok := a.alertCache[fp]
	if !ok && isResolved {
		// alert not found in alertmanager. must have been resolved.
		return alert.EndsAt.UTC(), true
	}
	return cachedAlert.EndsAt.UTC(), false
}

func (a *AlertStore) syncWithAlertmanager() error {
	filter := alertmanager.NewDefaultFilter()
	filter.IsSilenced = true
	alertList, err := a.alertmanagerClient.ListAlerts(filter)
	if err != nil {
		return errors.Wrap(err, "syncing with alertmanager failed")
	}

	m := make(map[model.Fingerprint]*client.ExtendedAlert)
	for _, alert := range alertList {
		fp, err := model.FingerprintFromString(alert.Fingerprint)
		if err != nil {
			a.logger.LogError("failed to create fingerprint for alert. ignoring", err)
			continue
		}
		m[fp] = alert
	}
	a.alertCache = m
	return nil
}

func (a *AlertStore) findAlertsInCache(labelSet client.LabelSet) ([]*client.ExtendedAlert, error) {
	result := make([]*client.ExtendedAlert, 0)
	for _, al := range a.alertCache {
		if util.LabelSetContains(al.Labels, labelSet) {
			result = append(result, al)
		}
	}
	//found at least one alert
	if len(result) > 0 {
		return result, nil
	}

	// if we get here the alert wasn't found. sync alerts and try again.
	err := a.syncWithAlertmanager()
	if err != nil {
		return nil, err
	}

	for _, al := range a.alertCache {
		if util.LabelSetContains(al.Labels, labelSet) {
			result = append(result, al)
		}
	}
	//found at least one alert
	if len(result) > 0 {
		return result, nil
	}

	return nil, ErrNotFound
}

// IsErrNotFound checks whether the error is an ErrNotFound
func IsErrNotFound(err error) bool {
	if err != nil {
		return err.Error() == ErrNotFound.Error()
	}
	return false
}
