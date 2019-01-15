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
	"encoding/gob"
	"fmt"
	"github.com/prometheus/alertmanager/client"
	"github.com/prometheus/common/model"
	"github.com/sapcc/stargate/pkg/log"
	"io"
	"math/rand"
	"os"
	"sync"
)

// FilePersister is used to save/load an AlertStore to/from a file.
type FilePersister struct {
	mtx      sync.RWMutex
	filePath string
	reader   io.Reader
	logger   log.Logger
}

// NewFilePersister returns a new FilePersister.
func NewFilePersister(filePath string, logger log.Logger) (*FilePersister, error) {
	p := &FilePersister{
		mtx:      sync.RWMutex{},
		filePath: filePath,
		logger:   log.NewLoggerWith(logger, "component", "FilePersister"),
	}

	f, err := p.openOrCreateFile()
	if err != nil {
		return nil, err
	}
	p.reader = f

	return p, nil
}

func (p *FilePersister) openOrCreateFile() (*os.File, error) {
	f, err := os.Open(p.filePath)
	if os.IsNotExist(err) {
		f, err := os.Create(p.filePath)
		if err != nil {
			return nil, err
		}
		p.logger.LogInfo("created persistence file", "file", p.filePath)
		return f, nil
	}
	p.logger.LogInfo("using existing persistence file", "file", p.filePath)
	return f, nil
}

// Load attempts to load a store from a file.
func (p *FilePersister) Load() (map[model.Fingerprint]*client.ExtendedAlert, error) {
	p.mtx.RLock()
	defer p.mtx.RUnlock()

	f, err := os.Open(p.filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	type alertList []*client.ExtendedAlert
	var l = new(alertList)
	d := gob.NewDecoder(f)
	if err := d.Decode(l); err != nil {
		return nil, err
	}

	store := make(map[model.Fingerprint]*client.ExtendedAlert)
	for _, alert := range *l {
		fp, err := model.FingerprintFromString(alert.Fingerprint)
		if err != nil {
			p.logger.LogError("error creating fingerprint for alert", err)
			continue
		}
		store[fp] = alert
	}
	return store, nil
}

// Store attempts to save a store to a file.
func (p *FilePersister) Store(store map[model.Fingerprint]*client.ExtendedAlert) (int64, error) {
	var size int64
	p.mtx.Lock()
	defer p.mtx.Unlock()

	tmpFilename := fmt.Sprintf("%s.%x", p.filePath, uint64(rand.Int63()))
	f, err := os.Create(tmpFilename)
	if err != nil {
		return size, err
	}
	defer f.Close()

	storeList := make([]*client.ExtendedAlert, 0)
	for _, alert := range store {
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
