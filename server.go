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
	"fmt"
	"github.com/aberic/gnomon"
	"github.com/aberic/gnomon/log"
	"golang.org/x/net/context"
)

// Server gRPC服务
type Server struct{}

// Heartbeat 接收发送心跳
func (s *Server) Heartbeat(ctx context.Context, req *ReqHeartBeat) (resp *RespHeartBeat, err error) {
	var (
		addr string
		port int
	)
	//log.Debug("raft", log.Field("receive heartbeat", req))
	if _, ok := raft.persistence.nodes[req.Id]; !ok {
		raft.persistence.appendNode(&Node{Id: req.Id, Url: req.Url, UnusualTimes: 0})
	}
	if addr, port, err = gnomon.GRPCGetClientIP(ctx); nil != err {
		return
	}
	resp = &RespHeartBeat{}
	hb := &heartBeat{
		ReqHeartBeat:  *req,
		clientAddress: addr,
		clientPort:    port,
	}
	raft.role.update(hb)
	return
}

// NodeList 接收请求集群节点集合
func (s *Server) NodeList(_ context.Context, req *ReqNodeList) (resp *RespNodeList, err error) {
	// fix 处理麻烦，待优化
	resp = &RespNodeList{Nodes: []*Node{}}
	for _, nodeLocal := range raft.persistence.nodes {
		have := false
		for _, nodeReq := range req.Nodes {
			if nodeReq.Id == nodeLocal.Id {
				have = true
			}
		}
		if !have {
			resp.Nodes = append(resp.Nodes, &Node{
				Id:           nodeLocal.Id,
				Url:          nodeLocal.Url,
				UnusualTimes: nodeLocal.UnusualTimes,
			})
		}
	}

	for _, node := range req.Nodes {
		if _, ok := raft.persistence.nodes[node.Id]; !ok {
			raft.persistence.appendNode(node)
		}
	}

	return
}

// Data 接收请求当前集群指定key数据
func (s *Server) Data(_ context.Context, req *ReqData) (resp *RespData, err error) {
	di, err := raft.persistence.data.get(req.Key)
	if nil != err {
		return nil, err
	}
	return &RespData{Value: di.value}, nil
}

// DataList 接收请求集群数据集合
func (s *Server) DataList(_ context.Context, _ *ReqDataList) (resp *RespDataList, err error) {
	resp = &RespDataList{DataArr: []*Data{}}
	for k, v := range raft.persistence.data.dataMap {
		resp.DataArr = append(resp.DataArr, &Data{Key: k, Hash: v.hash, Version: v.version})
	}
	return
}

// SyncData 接收同步数据
func (s *Server) SyncData(_ context.Context, req *ReqSyncData) (resp *RespSyncData, err error) {
	if err := raft.role.syncData(req); nil != err {
		return &RespSyncData{Term: raft.persistence.term, Success: false}, err
	}
	return &RespSyncData{Term: raft.persistence.term, Success: true}, nil
}

// Vote 接收发起选举，索要选票
func (s *Server) Vote(_ context.Context, req *ReqVote) (resp *RespVote, err error) {
	log.Info("raft", log.Field("vote", *req))
	if node, ok := raft.persistence.nodes[req.Id]; ok {
		if node.Url != req.Url {
			errStr := fmt.Sprintf(
				"cluster info error, now vote, req.id is %s, have.id is %s, req.url is %s, have.url is %s",
				req.Id, node.Id, req.Url, node.Url)
			return &RespVote{VoteGranted: false}, errors.New(errStr)
		}
	} else {
		raft.persistence.appendNode(&Node{Id: req.Id, Url: req.Url, UnusualTimes: 0})
	}
	if voteGranted, err := raft.role.vote(req); !voteGranted {
		return &RespVote{VoteGranted: false}, err
	}
	return &RespVote{VoteGranted: true}, nil
}
