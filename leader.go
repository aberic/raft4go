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
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/aberic/gnomon"
	"time"
)

// leader 负责接收客户端的请求，将日志复制到其他节点并告知其他节点何时应用这些日志是安全的
type leader struct {
	base      roleBase
	scheduled *time.Timer   // 定时发送心跳信息
	stop      chan struct{} // 释放当前角色chan
	ctx       context.Context
	cancel    context.CancelFunc
}

// work 开始本职工作
func (l *leader) start() {
	gnomon.Log().Info("raft", gnomon.Log().Field("leader", "start"), gnomon.Log().Field("term", raft.persistence.term))
	l.base.setStatus(RoleStatusLeader)
	l.scheduled = time.NewTimer(time.Millisecond * time.Duration(timeout))
	l.stop = make(chan struct{}, 1)
	l.ctx, l.cancel = context.WithCancel(context.Background())
	go l.heartbeats()
}

// update 更新状态
func (l *leader) update(hb *heartBeat) {
	if hb.Term == raft.persistence.term { // 两个节点任期相同，则重新开始新一轮选举，自身变成候选人角色
		raft.tuneCandidate()
	} else if hb.Term > raft.persistence.term { // 当前任期小于对方节点，自身变成跟随节点
		raft.tuneFollower(hb)
	} else { // 当前任期大于对方节点，检查对方节点是否被标记成无效节点
		if node, ok := raft.persistence.nodes[hb.Id]; ok {
			if node.UnusualTimes >= 3 { // 如果该节点的无效次数大于等于3，重置该节点有效性
				node.UnusualTimes = 0
			}
		} else { // 如果本地集群无此节点，则新增该节点，后续心跳会发送至该节点
			raft.persistence.appendNode(&Node{
				Id:           hb.Id,
				Url:          hb.Url,
				UnusualTimes: 0,
			})
		}
	}
}

// release 角色释放
func (l *leader) release() {
	gnomon.Log().Info("raft", gnomon.Log().Field("leader", "release"))
	l.cancel()
	l.stop <- struct{}{} // 关闭检查leader节点是否状态超时
	l.scheduled.Stop()
	//l.ctx = nil
	//l.cancel = nil
}

// put 角色所属集群新增数据
func (l *leader) put(key string, value []byte) error {
	if gnomon.String().IsEmpty(key) {
		return errors.New("key can't be empty")
	}
	if nil == value {
		return errors.New("value can't be nil")
	}
	var version int32
	if di, err := raft.persistence.data.get(key); nil == err {
		if bytes.Equal(value, di.value) {
			return nil
		}
		version = di.version + 1
	}
	// todo 流程待正规化
	for _, node := range raft.persistence.nodes {
		go func() {
			_ = reqSyncData(context.Background(), node, &ReqSyncData{
				Term:      raft.persistence.term,
				LeaderId:  raft.persistence.node.Id,
				LeaderUrl: raft.persistence.node.Url,
				Key:       key,
				Value:     value,
				Version:   version,
			})
		}()
	}
	raft.persistence.data.put(key, value, version)
	return nil
}

// syncData 请求同步数据
func (l *leader) syncData(req *ReqSyncData) error {
	if req.LeaderId != raft.persistence.node.Id {
		return fmt.Errorf("cluster status error, now is leader, req.leader.id is %s, raft.leader.id is %s", req.LeaderId, raft.persistence.leader.Id)
	}
	if di, err := raft.persistence.data.get(req.Key); nil == err {
		if di.version > req.Version {
			return errors.New("version less-than")
		}
		if bytes.Equal(req.Value, di.value) && req.Version == di.version {
			return errors.New("data immutability")
		}
		di.version = req.Version - 1
	}
	return l.put(req.Key, req.Value)
}

// vote 接收请求投票数据
func (l *leader) vote(req *ReqVote) (bool, error) {
	if req.Term >= raft.persistence.term {
		if raft.persistence.votedFor.set(req.Id, req.Term, req.Timestamp) {
			return true, nil
		}
		return false, fmt.Errorf("term %v less-than %v, or timestamp %v less-than %v",
			req.Term, raft.persistence.votedFor.term, req.Timestamp, raft.persistence.votedFor.timestamp)
	}
	return false, fmt.Errorf("term %v less-than %v", req.Term, raft.persistence.votedFor.term)
}

// roleStatus 获取角色状态
func (l *leader) roleStatus() RoleStatus {
	return l.base.status
}

// heartbeats 向节点集合发送心跳
func (l *leader) heartbeats() {
	l.scheduled.Reset(time.Millisecond * time.Duration(timeHeartbeat))
	for {
		select {
		case <-l.scheduled.C:
			in := &ReqHeartBeat{
				Term: raft.persistence.term,
				Id:   raft.persistence.node.Id,
				Url:  raft.persistence.node.Url,
				Hash: raft.persistence.data.hash,
			}
			raft.persistence.nodesLock.RLock()
			for _, node := range raft.persistence.nodes {
				go reqHeartbeat(l.ctx, node, in)
			}
			raft.persistence.nodesLock.RUnlock()
			l.scheduled.Reset(time.Millisecond * time.Duration(timeHeartbeat))
		case <-l.stop:
			return
		}
	}
}
