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
	"os"
	"strings"
	"sync"
)

// export GOPROXY=https://goproxy.io
// export GO111MODULE=on

const (
	k8sEnv            = "RAFT_K8S"               // K8S=true
	brokerID          = "RAFT_BROKER_ID"         // BROKER_ID=1
	nodeAddr          = "RAFT_NODE_ADDRESS"      // NODE_ADDRESS=example.com NODE_ADDRESS=127.0.0.1:19865:19877
	cluster           = "RAFT_CLUSTER"           // CLUSTER=1=127.0.0.1:19865:19877,2=127.0.0.2:19865:19877,3=127.0.0.3:19865:19877
	timeHeartbeatEnv  = "RAFT_TIME_HEARTBEAT"    // raft心跳定时时间ms
	timeCheckEnv      = "RAFT_TIME_CHECK"        // raft心跳定时检查超时时间ms
	timeoutEnv        = "RAFT_TIMEOUT"           // raft心跳超时ms
	portEnv           = "RAFT_PORT"              // raft服务开放端口号，默认19877
	logDirEnv         = "RAFT_LOG_DIR"           // 日志文件目录
	logFileMaxSizeEnv = "RAFT_LOG_FILE_MAX_SIZE" // 每个日志文件保存的最大尺寸 单位：M
	logFileMaxAgeEnv  = "RAFT_LOG_FILE_MAX_AGE"  // 文件最多保存多少天
	logUtcEnv         = "RAFT_LOG_UTC"           // CST & UTC 时间
	logLevelEnv       = "RAFT_LOG_LEVEL"         // 日志级别(debugLevel/infoLevel/warnLevel/ErrorLevel/panicLevel/fatalLevel)
	logProductionEnv  = "RAFT_LOG_PRODUCTION"    // 是否生产环境，在生产环境下控制台不会输出任何日志
)

func init() {
	timeHeartbeat = gnomon.Env().GetInt64D(timeHeartbeatEnv, 1000)
	timeCheck = gnomon.Env().GetInt64D(timeCheckEnv, 1500)
	timeout = gnomon.Env().GetInt64D(timeoutEnv, 2000)
	port = gnomon.Env().GetD(portEnv, "19877")
	logFileDir = gnomon.Env().GetD(logDirEnv, os.TempDir())
	logFileMaxSize = gnomon.Env().GetIntD(logFileMaxSizeEnv, 1024)
	logFileMaxAge = gnomon.Env().GetIntD(logFileMaxAgeEnv, 7)
	logUtc = gnomon.Env().GetBool(logUtcEnv)
	logLevel = gnomon.Env().GetD(logLevelEnv, "Debug")
	logProduction = gnomon.Env().GetBool(logProductionEnv)
}

var (
	raft           *Raft     // raft 实例
	once           sync.Once // once 确保Raft的启动方法只会被调用一次
	timeHeartbeat  int64     // raft心跳定时ms
	timeCheck      int64     // raft心跳定时检查超时时间
	timeout        int64     // raft心跳超时ms
	port           string    // raft服务开放端口号，默认19877
	logFileDir     string    // 日志文件目录
	logFileMaxSize int       // 每个日志文件保存的最大尺寸 单位：M
	logFileMaxAge  int       // 文件最多保存多少天
	logUtc         bool      // CST & UTC 时间
	logLevel       string    // 日志级别(debugLevel/infoLevel/warnLevel/ErrorLevel/panicLevel/fatalLevel)
	logProduction  bool      // 是否生产环境，在生产环境下控制台不会输出任何日志
)

type Params struct {
	Node          *Node   // 自身节点信息
	Nodes         []*Node // 集群节点信息
	TimeHeartbeat int64   // raft心跳定时ms
	TimeCheckReq  int64   //  raft心跳定时检查超时时间ms
	TimeoutReq    int64   // raft心跳定时ms
	PortReq       string  // raft服务开放端口号，默认19877
	Log           *Log    // 日志
}

type Log struct {
	Dir         string // 日志文件目录
	FileMaxSize int    // 每个日志文件保存的最大尺寸 单位：M
	FileMaxAge  int    // 文件最多保存多少天
	Utc         bool   // CST & UTC 时间
	Level       string // 日志级别(debug/info/warn/error/panic/fatal)
	Production  bool   // 是否生产环境，在生产环境下控制台不会输出任何日志
}

func initLog(log *Log) error {
	logLevel = "debug"
	if nil != log {
		if gnomon.String().IsNotEmpty(log.Dir) {
			logFileDir = log.Dir
		}
		if log.FileMaxSize > 0 {
			logFileMaxSize = log.FileMaxSize
		}
		if log.FileMaxAge > 0 {
			logFileMaxAge = log.FileMaxAge
		}
		logUtc = log.Utc
		logProduction = log.Production
		logLevel = log.Level
	}
	if err := gnomon.Log().Init(logFileDir, logFileMaxSize, logFileMaxAge, logUtc); nil != err {
		return err
	}
	var level gnomon.Level
	switch strings.ToLower(logLevel) {
	case "debug":
		level = gnomon.Log().DebugLevel()
	case "info":
		level = gnomon.Log().InfoLevel()
	case "warn":
		level = gnomon.Log().WarnLevel()
	case "error":
		level = gnomon.Log().ErrorLevel()
	case "panic":
		level = gnomon.Log().PanicLevel()
	case "fatal":
		level = gnomon.Log().FatalLevel()
	default:
		level = gnomon.Log().DebugLevel()
	}
	gnomon.Log().Set(level, logProduction)
	return nil
}

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
	if err := initLog(nil); nil != err {
		gnomon.Log().Error("raft", gnomon.Log().Err(err))
		return
	}
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
	if gnomon.String().IsNotEmpty(params.PortReq) {
		port = params.PortReq
	}
	gnomon.Log().Info("raft", gnomon.Log().Field("new", "new instance raft"))
	if err := initLog(params.Log); nil != err {
		gnomon.Log().Error("raft", gnomon.Log().Err(err))
		return
	}
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
func NodeList() map[string]*nodal {
	nodalList := raft.persistence.nodes
	nodalList[raft.persistence.node.Id] = raft.persistence.node
	return nodalList
}
