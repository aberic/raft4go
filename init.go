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

// raft
//
// 所有节点初始状态都是Follower角色
//
// 超时时间内没有收到Leader的请求则转换为Candidate进行选举
//
// Candidate收到大多数节点的选票则转换为Leader；发现Leader或者收到更高任期的请求则转换为Follower
//
// Leader在收到更高任期的请求后转换为Follower
//
// Raft把时间切割为任意长度的任期（term），每个任期都有一个任期号，采用连续的整数

package raft4go

import (
	"github.com/aberic/gnomon"
	"google.golang.org/grpc"
	"net"
	"sync"
)

// export GOPROXY=https://goproxy.io
// export GO111MODULE=on

const (
	k8s          = "K8S"          // K8S=true
	brokerID     = "BROKER_ID"    // BROKER_ID=1
	nodeAddr     = "NODE_ADDRESS" // NODE_ADDRESS=example.com NODE_ADDRESS=127.0.0.1:19865:19877
	cluster      = "CLUSTER"      // CLUSTER=1=127.0.0.1:19865:19877,2=127.0.0.2:19865:19877,3=127.0.0.3:19865:19877
	timeCheckEnv = "TIME_CHECK"   // raft心跳定时检查超时时间
	timeoutEnv   = "TIMEOUT"      // raft心跳定时/超时ms
	portEnv      = "PORT"         // raft服务开放端口号，默认19877
)

func init() {
	timeCheck = gnomon.Env().GetInt64D(timeCheckEnv, 1000)
	timeout = gnomon.Env().GetInt64D(timeoutEnv, 1500)
	port = gnomon.Env().GetD(portEnv, "19877")
}

var (
	raft      *Raft     // raft 实例
	once      sync.Once // once 确保Raft的启动方法只会被调用一次
	timeCheck int64     // raft心跳定时检查超时时间
	timeout   int64     // raft心跳定时/超时ms
	port      string    // raft服务开放端口号，默认19877
)

func gRPCListener() {
	var (
		listener net.Listener
		err      error
	)
	gnomon.Log().Info("raft", gnomon.Log().Field("gRPC", "start"), gnomon.Log().Field("port", port))
	//  创建server端监听端口
	if listener, err = net.Listen("tcp", gnomon.String().StringBuilder(":", port)); nil != err {
		panic(err)
	}
	//  创建gRPC的server
	rpcServer := grpc.NewServer()
	// 注册自定义服务
	RegisterRaftServer(rpcServer, &Server{})
	//  启动gRPC服务
	if err = rpcServer.Serve(listener); nil != err {
		panic(err)
	}
	gnomon.Log().Warn("raft", gnomon.Log().Field("gRPC", err))
}

// RaftStart 启动且只能启动一次Raft服务
func RaftStart() {
	gnomon.Log().Info("raft", gnomon.Log().Field("new", "new instance raft"))
	once.Do(func() {
		go gRPCListener()
		raft = &Raft{}
		raft.start()
	})
}

// RaftStartWithParams 启动且只能启动一次Raft服务
//
// node 自身节点信息
//
// nodes 集群节点信息
//
// timeCheck  raft心跳定时检查超时时间
//
// timeout raft心跳定时/超时ms
func RaftStartWithParams(node *Node, nodes []*Node, timeCheckReq, timeoutReq int64, portReq string) {
	if timeCheckReq != 0 {
		timeCheck = timeCheckReq
	}
	if timeoutReq != 0 {
		timeout = timeoutReq
	}
	if gnomon.String().IsNotEmpty(portReq) {
		port = portReq
	}
	gnomon.Log().Info("raft", gnomon.Log().Field("new", "new instance raft"))
	once.Do(func() {
		go gRPCListener()
		raft = &Raft{}
		raft.startWithParams(node, nodes)
	})
}

// Status 获取角色状态，0-leader、1-candidate、2-follower
//
// RoleStatusLeader、RoleStatusCandidate、RoleStatusFollower
func Status() RoleStatus {
	return raft.role.roleStatus()
}

// Put 集群新增数据
func Put(key string, value []byte) error {
	return raft.put(key, value)
}

// Get 从集群获取数据
func Get(key string) ([]byte, error) {
	return raft.get(key)
}

// NodeList 查看当前集群中节点集合，包括自身
func NodeList() []*nodal {
	return append(raft.persistence.nodes, raft.persistence.node)
}
