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

import "sync"

// votedFor 投票结果
type votedFor struct {
	id        string // 在当前获得选票的候选人的 Id
	term      int32  // 在当前获得选票的候选人的任期
	timestamp int64  // 在当前获取选票的候选人时间戳
}

func (v *votedFor) set(id string, term int32, timestamp int64) {
	v.id = id
	v.term = term
	v.timestamp = timestamp
}

// persistence 所有角色都拥有的持久化的状态（在响应RPC请求之前变更且持久化的状态）
type persistence struct {
	leader    *Node        // 当前任务Leader
	term      int32        // 服务器的任期，初始为0，递增
	node      *Node        // 自身节点信息
	nodes     []*Node      // 当前Raft可见节点集合
	nodesLock sync.RWMutex // 当前Raft可见节点集合锁
	votedFor  *votedFor    // 在当前获得选票的候选人的 Id
	data      *data        // raft数据内容
}

func (p *persistence) setLeader(id, url string) {
	p.leader.Id = id
	p.leader.Url = url
}

func (p *persistence) appendNode(node *Node) {
	defer p.nodesLock.Unlock()
	p.nodesLock.Lock()
	p.nodes = append(p.nodes, node)
}

func (p *persistence) appendNodes(nodeList []*Node) {
	defer p.nodesLock.Unlock()
	p.nodesLock.Lock()
	p.nodes = append(p.nodes, nodeList...)
}

func (p *persistence) Nodes() []*Node {
	defer p.nodesLock.RUnlock()
	p.nodesLock.RLock()
	nodes := p.nodes
	return nodes
}
