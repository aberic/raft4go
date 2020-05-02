/*
 * Copyright (c) 2020. Aberic - All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 * http://www.apache.org/licenses/LICENSE-2.0
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package raft4go

import (
	"fmt"
	"github.com/aberic/gnomon"
	"sort"
	"sync"
)

// 数据详细对象
type dataInfo struct {
	value   []byte // 数据值
	hash    string // value数据的散列值
	version int32
	lock    sync.RWMutex
}

type data struct {
	dataMap  map[string]*dataInfo // 数据详细对象集合
	hash     string               // 所有数据集hash，用于比较彼此数据
	lock     sync.RWMutex
	hashLock sync.Mutex
}

func (d *data) put(key string, value []byte, version int32) {
	if _, ok := d.dataMap[key]; ok {
		if d.dataMap[key].version < version {
			defer d.dataMap[key].lock.Unlock()
			d.dataMap[key].lock.Lock()
			d.dataMap[key].value = value
			d.dataMap[key].version = version
			d.dataMap[key].hash = gnomon.HashMD5Bytes(value)
		}
	} else {
		defer d.lock.Unlock()
		d.lock.Lock()
		d.dataMap[key] = &dataInfo{
			lock:    sync.RWMutex{},
			value:   value,
			version: version,
			hash:    gnomon.HashMD5Bytes(value),
		}
	}
	d.updateHash()
}

func (d *data) get(key string) (dataInfo *dataInfo, err error) {
	if _, ok := d.dataMap[key]; ok {
		defer d.dataMap[key].lock.RUnlock()
		d.dataMap[key].lock.RLock()
		return d.dataMap[key], nil
	}
	return nil, fmt.Errorf("value of key is not exist")
}

func (d *data) updateHash() {
	defer d.hashLock.Unlock()
	d.hashLock.Lock()
	var vs []string
	for _, v := range d.dataMap {
		vs = append(vs, v.hash)
	}
	sort.Strings(vs)
	var hashStr string
	for _, v := range vs {
		hashStr = gnomon.StringBuild(hashStr, v)
	}
	d.hash = gnomon.HashMD5(hashStr)
}
