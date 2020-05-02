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
	"strings"
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

// Start Raft启用方法
func (r *Raft) start() {
	r.once.Do(func() {
		r.init()
		log.Info("raft init success")
		r.initEnv()
		log.Info("raft env init success")
		r.initRole()
		log.Info("raft role init success")
	})
}

// Start Raft启用方法
//
// node 自身节点信息
//
// nodes 集群节点信息
func (r *Raft) startWithParams(node *Node, nodes []*Node) {
	if nil == node || nil == nodes {
		log.Error("raft", log.Field("describe", "startWithParams fail"),
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
		leader: &nodal{},
		term:   0,
		node:   &nodal{},
		nodes:  map[string]*nodal{},
		votedFor: &votedFor{
			id:        "",
			term:      0,
			timestamp: time.Now().UnixNano(),
		},
		data: &data{dataMap: make(map[string]*dataInfo)},
	}
}

// initEnv raft环境变量初始化
func (r *Raft) initEnv() {
	// 仅测试用
	//_ = os.Setenv(brokerID, "1")
	//_ = os.Setenv(nodeAddr, "127.0.0.1:19880")
	//_ = os.Setenv(cluster, "1=127.0.0.1:19877,2=127.0.0.1:19878,3=127.0.0.1:19879")
	if k8s := gnomon.EnvGetBool(k8sEnv); k8s {
		if r.persistence.node.Url = gnomon.EnvGet("HOSTNAME"); gnomon.StringIsEmpty(r.persistence.node.Url) {
			log.Error("raft", log.Field("describe", "init with k8s fail"),
				log.Field("addr", r.persistence.node.Url))
			return
		}
		r.persistence.node.Id = strings.Split(r.persistence.node.Url, "-")[1]
		log.Info("raft", log.Field("describe", "init with k8s"),
			log.Field("addr", r.persistence.node.Url), log.Field("id", r.persistence.node.Id))
	} else {
		if r.persistence.node.Url = gnomon.EnvGet(nodeAddrEnv); gnomon.StringIsEmpty(r.persistence.node.Url) {
			log.Error("raft", log.Field("describe", "init with env fail"),
				log.Errs("NODE_ADDRESS is empty"))
			return
		}
		if r.persistence.node.Id = gnomon.EnvGet(brokerIDEnv); gnomon.StringIsEmpty(r.persistence.node.Id) {
			log.Error("raft", log.Field("describe", "init with env fail"),
				log.Errs("broker id is not appoint"))
			return
		}
		log.Info("raft", log.Field("describe", "init with env"),
			log.Field("addr", r.persistence.node.Url), log.Field("id", r.persistence.node.Id))
	}
	raft.persistence.node.UnusualTimes = -1
	r.initCluster(gnomon.EnvGet(clusterEnv))
}

// initEnv raft环境变量初始化
func (r *Raft) initWithParams(node *Node, nodes []*Node) {
	raft.persistence.node = &nodal{Node: *node, pool: nil}
	raft.persistence.node.UnusualTimes = -1
	raft.persistence.appendNodes(nodes)
}

// initCluster 初始化集群节点
func (r *Raft) initCluster(nodesStr string) {
	if gnomon.StringIsEmpty(nodesStr) {
		nodesStr = gnomon.EnvGet(clusterEnv)
	}
	log.Info("raft", log.Field("node cluster", nodesStr))
	if gnomon.StringIsNotEmpty(nodesStr) {
		clusterArr := strings.Split(nodesStr, ",")
		for _, cluster := range clusterArr {
			clusterSplit := strings.Split(cluster, "=")
			id := clusterSplit[0]
			if gnomon.StringIsEmpty(id) {
				log.Error("raft", log.Field("describe", "init with env fail"),
					log.Errs("one of cluster's broker id is nil"))
				continue
			}
			if id == r.persistence.node.Id {
				continue
			}
			nodeURL := clusterSplit[1]
			r.persistence.appendNode(&Node{Id: id, Url: nodeURL, UnusualTimes: 0})
		}
	}
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
