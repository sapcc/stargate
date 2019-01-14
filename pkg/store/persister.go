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
	"fmt"
	"io"
	"math/rand"
	"os"
	"sync"
	"time"

	"encoding/gob"
	"github.com/prometheus/alertmanager/store"
	"github.com/prometheus/alertmanager/types"
	"github.com/sapcc/stargate/pkg/log"
)

type filePersister struct {
	mtx        sync.RWMutex
	filePath   string
	gcInterval time.Duration
	reader     io.Reader
	logger     log.Logger
}

func NewFilePersister(filePath string, gcInterval time.Duration, logger log.Logger) (*filePersister, error) {
	p := &filePersister{
		filePath:   filePath,
		gcInterval: gcInterval,
		logger:     log.NewLoggerWith(logger, "component", "filePersister"),
	}

	f, err := p.openOrCreateFile()
	if err != nil {
		return nil, err
	}
	p.reader = f

	return p, nil
}

func (p *filePersister) openOrCreateFile() (*os.File, error) {
	var (
		f   *os.File
		err error
	)

	f, err = os.Open(p.filePath)
	if err != nil {
		if os.IsExist(err) {
			f, err = os.Create(p.filePath)
			if err != nil {
				return nil, err
			}
		}
	}
	return f, err
}

func (p *filePersister) Load() (*store.Alerts, error) {
	p.mtx.RLock()
	defer p.mtx.RUnlock()

	f, err := os.Open(p.filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	type alertList []*types.Alert
	var l = new(alertList)
	d := gob.NewDecoder(f)
	if err := d.Decode(l); err != nil {
		return nil, err
	}

	alertStore := store.NewAlerts(p.gcInterval)
	for _, alert := range *l {
		if err := alertStore.Set(alert); err != nil {
			p.logger.LogError("error loading alert", err)
		}
	}
	return alertStore, nil
}

func (p *filePersister) Store(alertStore *store.Alerts) (int64, error) {
	var size int64
	p.mtx.Lock()
	defer p.mtx.Unlock()

	tmpFilename := fmt.Sprintf("%s.%x", p.filePath, uint64(rand.Int63()))
	f, err := os.Create(tmpFilename)
	if err != nil {
		return size, err
	}
	defer f.Close()

	storeList := make([]*types.Alert, 0)
	for alert := range alertStore.List() {
		storeList = append(storeList, alert)
	}

	e := gob.NewEncoder(f)
	if err := e.Encode(storeList); err != nil {
		p.logger.LogError("error encoding alert store", err)
	}

	stat, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return stat.Size(), os.Rename(tmpFilename, p.filePath)
}
