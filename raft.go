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
	"errors"
	"github.com/aberic/gnomon"
	"github.com/aberic/gnomon/log"
	"sync"
	"time"
)

// Raft 接收客户端提交的同步内容，被封装在自定义的方法中
//
// 也返回客户端期望的同步结果及从其他节点同步过来的信息
type Raft struct {
	persistence    *persistence // persistence 持久化的状态（在响应RPC请求之前变更且持久化的状态）
	role           role         // raft当前角色
	once           sync.Once    // 确保Raft的启动方法只会被调用一次
	roleChangeLock sync.Mutex
}

// start Raft启用方法
//
// node 自身节点信息
//
// nodes 集群节点信息
func (r *Raft) start(node *Node, nodes []*Node) {
	if nil == node || nil == nodes {
		log.Error("raft", log.Field("describe", "start fail"),
			log.Errs("params can not be nil"))
		return
	}
	r.once.Do(func() {
		r.init()
		log.Info("raft init success")
		r.initWithParams(node, nodes)
		log.Info("raft params init success")
		r.initRole()
		log.Info("raft role init success")
	})
}

// init raft结构初始化
func (r *Raft) init() {
	r.persistence = &persistence{
		leader: &Node{},
		term:   0,
		node:   &Node{},
		nodes:  map[string]*Node{},
		votedFor: &votedFor{
			id:        "",
			term:      0,
			timestamp: time.Now().UnixNano(),
		},
		data: &data{dataMap: make(map[string]*dataInfo)},
	}
}

// initEnv raft环境变量初始化
func (r *Raft) initWithParams(node *Node, nodes []*Node) {
	raft.persistence.node = node
	raft.persistence.node.UnusualTimes = -1
	raft.persistence.appendNodes(nodes)
}

var (
	lead   *leader
	follow *follower
	candi  *candidate
)

// initRole raft角色初始化
func (r *Raft) initRole() {
	log.Info("raft", log.Field("initRole", "init raft role"))
	lead = &leader{}
	follow = &follower{}
	candi = &candidate{}
	r.tuneFollower(nil)
}

// tuneLeader 切换角色为leader
func (r *Raft) tuneLeader() {
	defer r.roleChangeLock.Unlock()
	r.roleChangeLock.Lock()
	if nil != r.role && r.role.roleStatus() == RoleStatusLeader {
		return
	}
	r.role.release()
	r.role = lead
	r.role.start()
}

// tuneLeader 切换角色为follower
func (r *Raft) tuneFollower(hb *heartBeat) {
	defer r.roleChangeLock.Unlock()
	r.roleChangeLock.Lock()
	if nil != r.role {
		if r.role.roleStatus() != RoleStatusFollower {
			r.role.release()
		} else {
			return
		}
	}
	r.role = follow
	r.role.start()
	if nil != hb {
		r.role.update(hb)
	}
}

// tuneLeader 切换角色为candidate
func (r *Raft) tuneCandidate() {
	defer r.roleChangeLock.Unlock()
	r.roleChangeLock.Lock()
	if nil != r.role && r.role.roleStatus() == RoleStatusCandidate {
		return
	}
	r.role.release()
	r.role = candi
	r.role.start()
}

// put 集群新增数据
func (r *Raft) put(key string, value []byte) error {
	return r.role.put(key, value)
}

// get 从集群获取数据
func (r *Raft) get(key string) ([]byte, error) {
	if gnomon.StringIsEmpty(key) {
		return nil, errors.New("key can't be empty")
	}
	dataInfo, err := raft.persistence.data.get(key)
	if nil == err {
		return dataInfo.value, nil
	}
	return nil, err
}
