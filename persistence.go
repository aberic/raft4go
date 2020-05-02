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
	"github.com/aberic/gnomon"
	"google.golang.org/grpc"
	"sync"
	"time"
)

// votedFor 投票结果
type votedFor struct {
	id        string // 在当前获得选票的候选人的 Id
	term      int32  // 在当前获得选票的候选人的任期
	timestamp int64  // 在当前获取选票的候选人时间戳
	lock      sync.Mutex
}

// set 投票设置成功与否
func (v *votedFor) set(id string, term int32, timestamp int64) bool {
	defer v.lock.Unlock()
	v.lock.Lock()
	if v.term < term {
		v.id = id
		v.term = term
		v.timestamp = timestamp
		return true
	} else if v.term == term {
		if v.timestamp > timestamp {
			v.id = id
			v.timestamp = timestamp
			return true
		} else if v.timestamp == timestamp {
			v.id = id
			return true
		}
	}
	return false
}

// persistence 所有角色都拥有的持久化的状态（在响应RPC请求之前变更且持久化的状态）
type persistence struct {
	leader    *nodal            // 当前任务Leader
	term      int32             // 服务器的任期，初始为0，递增
	node      *nodal            // 自身节点信息
	nodes     map[string]*nodal // 当前Raft可见节点集合
	nodesLock sync.RWMutex      // 当前Raft可见节点集合锁
	votedFor  *votedFor         // 在当前获得选票的候选人的 Id
	data      *data             // raft数据内容
}

func (p *persistence) setLeader(id, url string) {
	p.leader.Id = id
	p.leader.Url = url
}

func (p *persistence) appendNode(node *Node) {
	defer p.nodesLock.Unlock()
	p.nodesLock.Lock()
	if _, ok := p.nodes[node.Id]; ok {
		return
	}
	if nodal, err := newNode(node.Id, node.Url); nil == err {
		p.nodes[node.Id] = nodal
	}
}

func (p *persistence) appendNodes(nodeList []*Node) {
	defer p.nodesLock.Unlock()
	p.nodesLock.Lock()
	for _, node := range nodeList {
		if _, ok := p.nodes[node.Id]; ok {
			return
		}
		if nodal, err := newNode(node.Id, node.Url); nil == err {
			p.nodes[node.Id] = nodal
		}
	}
}

func (p *persistence) Nodes() []*Node {
	defer p.nodesLock.RUnlock()
	p.nodesLock.RLock()
	var nodes []*Node
	for _, nodal := range p.nodes {
		nodes = append(nodes, &Node{Id: nodal.Id, Url: nodal.Url})
	}
	return nodes
}

type nodal struct {
	Node
	pool *gnomon.Pond
}

func newNode(id, url string) (*nodal, error) {
	if p, err := gnomon.NewPond(10, 100, 5*time.Second, func() (c gnomon.Conn, err error) {
		return grpc.Dial(url, grpc.WithInsecure())
	}); nil != err {
		return nil, err
	} else {
		return &nodal{
			Node: Node{
				Id:           id,
				Url:          url,
				UnusualTimes: 0,
			},
			pool: p,
		}, nil
	}
}
