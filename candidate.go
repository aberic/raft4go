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
	"github.com/aberic/gnomon"
	"strconv"
	"sync"
	"time"
)

// candidate 用于选举Leader的一种角色
type candidate struct {
	base      roleBase
	timestamp int64 // 节点在成为候选身份后的时间戳
	ctx       context.Context
	cancel    context.CancelFunc
}

// work 开始本职工作
func (c *candidate) start() {
	gnomon.Log().Info("raft", gnomon.Log().Field("candidate", "start"), gnomon.Log().Field("term", raft.persistence.term))
	c.base.setStatus(RoleStatusCandidate)
	c.timestamp = time.Now().UnixNano()
	c.ctx, c.cancel = context.WithCancel(context.Background())
	go c.votes()
}

// update 更新状态
func (c *candidate) update(hb *heartBeat) {
	if hb.Term >= raft.persistence.term {
		raft.tuneFollower(hb)
	}
}

// release 角色释放
func (c *candidate) release() {
	gnomon.Log().Info("raft", gnomon.Log().Field("candidate", "release"))
	c.cancel()
	//c.ctx = nil
	//c.cancel = nil
}

// put 角色所属集群新增数据
func (c *candidate) put(key string, value []byte) error {
	if gnomon.String().IsEmpty(key) {
		return errors.New("key can't be empty")
	}
	if nil == value {
		return errors.New("value can't be nil")
	}
	return errors.New("cluster status error, now is candidate")
}

func (c *candidate) syncData(req *ReqSyncData) error {
	if req.Term >= raft.persistence.term { // 如果请求任期大于等于自身任期，则可能终止当前候选身份
		for _, node := range raft.persistence.nodes {
			if node.Id == req.LeaderId { // 如果请求leaderID存在于集群中，则自身变为follower节点
				raft.tuneFollower(nil)
				return nil
			}
		}
		// 没有结束，则表示该请求同步的leader节点为新发现节点
		raft.persistence.appendNode(&Node{Id: req.LeaderId, Url: req.LeaderUrl, UnusualTimes: 0})
		raft.persistence.setLeader(req.LeaderId, req.LeaderUrl)
		raft.tuneFollower(nil)
		return nil
	}
	return errors.New("cluster status error, now is candidate")
}

// roleStatus 获取角色状态
func (c *candidate) roleStatus() RoleStatus {
	return c.base.status
}

// votes 向节点集合发起投票
func (c *candidate) votes() {
	nodeCount := len(raft.persistence.nodes)
	votes := make(chan struct{}, nodeCount)
	wg := sync.WaitGroup{}
	for _, node := range raft.persistence.nodes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if voteGranted := reqVote(c.ctx, node, &ReqVote{
				Id:        raft.persistence.node.Id,
				Url:       raft.persistence.node.Url,
				Term:      raft.persistence.term + 1,
				Timestamp: c.timestamp,
			}); voteGranted {
				votes <- struct{}{}
			}
		}()
	}
	wg.Wait()
	gnomon.Log().Info("raft",
		gnomon.Log().Field("vote", gnomon.String().StringBuilder("len(votes)+1 = ",
			strconv.Itoa(len(votes)+1), " and nodeCount/2 = ", strconv.Itoa(nodeCount/2))))
	if len(votes)+1 > nodeCount/2 {
		raft.persistence.term += 1
		raft.persistence.setLeader(raft.persistence.node.Id, raft.persistence.node.Url)
		raft.tuneLeader()
	} else {
		if raft.role.roleStatus() == RoleStatusCandidate {
			raft.tuneFollower(nil)
		}
	}
}
