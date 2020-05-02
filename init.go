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
	"github.com/aberic/gnomon/log"
	"google.golang.org/grpc"
	"net"
	"os"
	"sync"
)

var (
	logFileDir     string // 日志文件目录
	logFileMaxSize int    // 每个日志文件保存的最大尺寸 单位：M
	logFileMaxAge  int    // 文件最多保存多少天
	logUtc         bool   // CST & UTC 时间
	logLevel       string // 日志级别(debugLevel/infoLevel/warnLevel/ErrorLevel/panicLevel/fatalLevel)
	logProduction  bool   // 是否生产环境，在生产环境下控制台不会输出任何日志
)

// export GOPROXY=https://goproxy.io
// export GO111MODULE=on

func init() {
	timeHeartbeat = gnomon.EnvGetInt64D(timeHeartbeatEnv, 1000)
	timeCheck = gnomon.EnvGetInt64D(timeCheckEnv, 1500)
	timeout = gnomon.EnvGetInt64D(timeoutEnv, 2000)
	port = gnomon.EnvGetD(portEnv, "19877")
	logFileDir = gnomon.EnvGetD(logDirEnv, os.TempDir())
	logFileMaxSize = gnomon.EnvGetIntD(logFileMaxSizeEnv, 1024)
	logFileMaxAge = gnomon.EnvGetIntD(logFileMaxAgeEnv, 7)
	logUtc = gnomon.EnvGetBool(logUtcEnv)
	logLevel = gnomon.EnvGetD(logLevelEnv, "Debug")
	logProduction = gnomon.EnvGetBool(logProductionEnv)
}

var (
	raft          *Raft     // raft 实例
	once          sync.Once // once 确保Raft的启动方法只会被调用一次
	timeHeartbeat int64     // raft心跳定时ms
	timeCheck     int64     // raft心跳定时检查超时时间
	timeout       int64     // raft心跳超时ms
	port          string    // raft服务开放端口号，默认19877
)

// Params 启动参数
type Params struct {
	Node          *Node   // 自身节点信息
	Nodes         []*Node // 集群节点信息
	TimeHeartbeat int64   // raft心跳定时ms
	TimeCheckReq  int64   //  raft心跳定时检查超时时间ms
	TimeoutReq    int64   // raft心跳定时ms
	PortReq       string  // raft服务开放端口号，默认19877
	Log           *Log    // 日志
}

// Log 日志属性
type Log struct {
	Dir         string // 日志文件目录
	FileMaxSize int    // 每个日志文件保存的最大尺寸 单位：M
	FileMaxAge  int    // 文件最多保存多少天
	Utc         bool   // CST & UTC 时间
	Level       string // 日志级别(debug/info/warn/error/panic/fatal)
	Production  bool   // 是否生产环境，在生产环境下控制台不会输出任何日志
}

func gRPCListener() {
	var (
		listener net.Listener
		err      error
	)
	log.Info("raft", log.Field("gRPC", "start"), log.Field("port", port))
	//  创建server端监听端口
	if listener, err = net.Listen("tcp", gnomon.StringBuild(":", port)); nil != err {
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
	log.Warn("raft", log.Field("gRPC", err))
}

// RaftStart 启动且只能启动一次Raft服务
func RaftStart() {
	log.Fit(logLevel, logFileDir, logFileMaxSize, logFileMaxAge, logUtc, logProduction)
	log.Info("raft", log.Field("new", "new instance raft"))
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
func RaftStartWithParams(params *Params) {
	if params.TimeHeartbeat != 0 {
		timeHeartbeat = params.TimeHeartbeat
	}
	if params.TimeCheckReq != 0 {
		timeCheck = params.TimeCheckReq
	}
	if params.TimeoutReq != 0 {
		timeout = params.TimeoutReq
	}
	if gnomon.StringIsNotEmpty(params.PortReq) {
		port = params.PortReq
	}
	log.Fit(params.Log.Level, params.Log.Dir, params.Log.FileMaxSize, params.Log.FileMaxAge, params.Log.Utc, params.Log.Production)
	log.Info("raft", log.Field("new", "new instance raft"))
	once.Do(func() {
		go gRPCListener()
		raft = &Raft{}
		raft.startWithParams(params.Node, params.Nodes)
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
func NodeList() Nodes {
	return raft.persistence.nodes
}
