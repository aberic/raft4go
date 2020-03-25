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
	"context"
	"errors"
	"fmt"
	"github.com/aberic/gnomon"
	"sync"
	"time"
)

// follower 负责响应来自Leader或者Candidate的请求
type follower struct {
	base      roleBase
	scheduled *time.Timer   // 超时检查对象
	time      int64         // 最后一次接收到心跳时间戳ms
	stop      chan struct{} // 释放当前角色chan
	synced    bool          // 是否已经同步过
	lock      sync.Mutex    // 同步操作锁
}

// work 开始本职工作
func (f *follower) start() {
	gnomon.Log().Info("raft", gnomon.Log().Field("follow", "start"))
	f.base.setStatus(RoleStatusFollower)
	f.scheduled = time.NewTimer(time.Millisecond * time.Duration(timeCheck))
	f.stop = make(chan struct{}, 1)
	f.refreshTime()
	f.checkTimeOut()
}

// update 更新状态
func (f *follower) update(hb *heartBeat) {
	// 任期相同，并且leader节点也匹配
	if hb.Term == raft.persistence.term && hb.Id == raft.persistence.leader.Id {
		f.refreshTime()
		if !f.synced {
			f.sync(hb)
		}
		// leader节点数据hash发生变更
		if hb.Hash != raft.persistence.data.hash {
			f.sync(hb)
		}
	} else if hb.Term > raft.persistence.term {
		f.refreshTime()
		f.sync(hb)
	}
}

// release 角色释放
func (f *follower) release() {
	gnomon.Log().Info("raft", gnomon.Log().Field("follow", "release"))
	f.stop <- struct{}{} // 关闭检查leader节点是否状态超时
	f.synced = false     // 重置同步状态
	f.scheduled.Stop()
}

// put 角色所属集群新增数据
func (f *follower) put(key string, value []byte) error {
	if gnomon.String().IsEmpty(key) {
		return errors.New("key can't be empty")
	}
	if nil == value {
		return errors.New("value can't be nil")
	}
	return reqSyncData(context.Background(), raft.persistence.leader.Url, &ReqSyncData{
		Term:      raft.persistence.term,
		LeaderId:  raft.persistence.leader.Id,
		LeaderUrl: raft.persistence.leader.Url,
		Version:   0,
		Key:       key,
		Value:     value,
	})
}

func (f *follower) syncData(req *ReqSyncData) error {
	if req.LeaderId != raft.persistence.leader.Id {
		errStr := fmt.Sprintf("cluster status error, now is follower, req.leader.id is %s, raft.leader.id is %s", req.LeaderId, raft.persistence.leader.Id)
		gnomon.Log().Warn("raft", gnomon.Log().Field("describe", errStr))
		return errors.New(errStr)
	}
	raft.persistence.data.put(req.Key, req.Value, req.Version)
	return nil
}

// roleStatus 获取角色状态
func (f *follower) roleStatus() RoleStatus {
	return f.base.status
}

// refreshTime 刷新心跳超时时间
func (f *follower) refreshTime() {
	f.time = time.Now().UnixNano() / 1e6
}

// checkTimeOut
//
// 检查leader节点是否状态超时
//
// leader节点会定时发送心跳，如果心跳间隔时间小于 timeout 则会判定超时
func (f *follower) checkTimeOut() {
	f.scheduled.Reset(time.Microsecond * time.Duration(timeCheck))
	for {
		select {
		case <-f.scheduled.C:
			if time.Now().UnixNano()/1e6-f.time > timeout {
				raft.tuneCandidate()
			} else {
				f.scheduled.Reset(time.Microsecond * time.Duration(timeCheck))
			}
		case <-f.stop:
			return
		}
	}
}

// sync 与leader同步节点寻求数据同步
func (f *follower) sync(hb *heartBeat) {
	defer f.lock.Unlock()
	f.lock.Lock()
	if hb.Id == raft.persistence.leader.Id { // 如果leader节点匹配
		if hb.Hash == raft.persistence.data.hash { // 如果对比持久化数据相同
			f.synced = true
			raft.persistence.term = hb.Term
			return
		}
	} else { // 如果leader节点不匹配
		if hb.Term > raft.persistence.term { // 如果任期大于本地任期
			if hb.Hash == raft.persistence.data.hash { // 如果对比持久化数据相同
				f.synced = true
				raft.persistence.setLeader(hb.Id, hb.Url) // 更新本地跟随leader信息
				raft.persistence.term = hb.Term
				return
			}
		} else { // 如果任期小于等于本地任期，什么也不做
			return
		}
	}
	f.syncCheck(hb)
}

// sync 与leader同步节点检查数据同步
func (f *follower) syncCheck(hb *heartBeat) {
	// 同步数据等相关操作
	ctx, cancel := context.WithCancel(context.Background())
	// 定义工作组，开启上下文管理
	wg := sync.WaitGroup{}
	// 请求集群节点集合
	wg.Add(1)
	go func() {
		defer wg.Done()
		nodeList, err := reqNodeList(ctx, hb.Url, &ReqNodeList{Nodes: raft.persistence.Nodes()})
		if err != nil {
			cancel()
		} else {
			raft.persistence.appendNodes(nodeList)
		}
	}()
	if hb.Hash != raft.persistence.data.hash {
		// 请求集群数据集合
		wg.Add(1)
		go func() {
			defer wg.Done()
			dataList, err := reqDataList(ctx, hb.Url, &ReqDataList{})
			if err != nil {
				cancel()
			} else {
				f.compareAndSwap(hb, hb.Url, dataList)
			}
		}()
	}
	wg.Wait()
}

// sync 与leader比较数据并更新数据
func (f *follower) compareAndSwap(hb *heartBeat, url string, dataList []*Data) {
	// 同步数据等相关操作
	errStruct := make(chan struct{}, len(dataList))
	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	for _, data := range dataList {
		di, err := raft.persistence.data.get(data.Key)
		if nil != err { // 如果本地数据中不存在当前数据，则新增
			wg.Add(1)
			go func() {
				defer wg.Done()
				bytes, err := reqData(ctx, url, &ReqData{Key: data.Key})
				if err != nil {
					errStruct <- struct{}{}
					cancel()
				} else {
					raft.persistence.data.put(data.Key, bytes, data.Version)
				}
			}()
		} else if data.Hash != di.hash { // 如果存在当前数据，则比较数据hash是否一致
			// 如果leader节点的数据版本号大于等于本地数据版本，则更新本地数据信息
			if data.Version >= di.version {
				wg.Add(1)
				go func() {
					defer wg.Done()
					bytes, err := reqData(ctx, url, &ReqData{Key: data.Key})
					if err != nil {
						errStruct <- struct{}{}
						cancel()
					} else {
						raft.persistence.data.put(data.Key, bytes, data.Version)
					}
				}()
			} else { // 如果leader节点的数据版本号小于本地数据版本，则通知leader更新数据
				wg.Add(1)
				go func() {
					defer wg.Done()
					if err := reqSyncData(ctx, url, &ReqSyncData{
						Term:      raft.persistence.term,
						LeaderId:  raft.persistence.leader.Id,
						LeaderUrl: raft.persistence.leader.Url,
						Version:   di.version,
						Key:       data.Key,
						Value:     di.value,
					}); err != nil {
						cancel()
					}
				}()
			}
		}
	}
	wg.Wait()
	if len(errStruct) == 0 {
		f.synced = true
		raft.persistence.setLeader(hb.Id, hb.Url) // 更新本地跟随leader信息
		raft.persistence.term = hb.Term
	}
	f.refreshTime()
}
