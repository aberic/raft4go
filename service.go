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
	"github.com/aberic/gnomon"
	"github.com/aberic/gnomon/log"
	"google.golang.org/grpc"
)

// reqHeartbeat 发送心跳
func reqHeartbeat(ctx context.Context, node *Node, in *ReqHeartBeat) {
	var resultChan = make(chan struct{})
	go func() {
		if _, err := gnomon.GRPCRequestPools(node.Url, func(conn *grpc.ClientConn) (interface{}, error) {
			// 创建grpc客户端
			cli := NewRaftClient(conn)
			//客户端向grpc服务端发起请求
			return cli.Heartbeat(context.Background(), in)
		}); nil != err {
			log.Warn("raft", log.Err(err))
			node.UnusualTimes++
			return
		}
		resultChan <- struct{}{}
	}()
	select {
	case <-ctx.Done():
		// 其他RPC调用调用失败
		return
	case <-resultChan:
		// 本RPC调用成功，不返回错误信息
		return
	}
}

// reqNodeList 请求集群节点集合
func reqNodeList(ctx context.Context, node *Node, in *ReqNodeList) ([]*Node, error) {
	var (
		resultChan = make(chan []*Node)
		errChan    = make(chan error)
	)
	go func() {
		var (
			resp interface{}
			err  error
		)
		resp, err = gnomon.GRPCRequestPools(node.Url, func(conn *grpc.ClientConn) (interface{}, error) {
			// 创建grpc客户端
			cli := NewRaftClient(conn)
			//客户端向grpc服务端发起请求
			return cli.NodeList(context.Background(), in)
		})
		if nil != err {
			log.Warn("raft", log.Err(err))
			errChan <- err
		} else {
			resultChan <- resp.(*RespNodeList).Nodes
		}
	}()
	select {
	case <-ctx.Done():
		// 其他RPC调用调用失败
		return nil, ctx.Err()
	case err := <-errChan:
		// 本RPC调用失败，返回错误信息
		return nil, err
	case nodeList := <-resultChan:
		// 本RPC调用成功，不返回错误信息
		return nodeList, nil
	}
}

// reqDataList 请求集群数据集合
func reqDataList(ctx context.Context, node *Node, in *ReqDataList) ([]*Data, error) {
	var (
		resultChan = make(chan []*Data)
		errChan    = make(chan error)
	)
	go func() {
		var (
			resp interface{}
			err  error
		)
		resp, err = gnomon.GRPCRequestPools(node.Url, func(conn *grpc.ClientConn) (interface{}, error) {
			// 创建grpc客户端
			cli := NewRaftClient(conn)
			//客户端向grpc服务端发起请求
			return cli.DataList(context.Background(), in)
		})
		if nil != err {
			log.Warn("raft", log.Err(err))
			errChan <- err
		} else {
			resultChan <- resp.(*RespDataList).DataArr
		}
	}()
	select {
	case <-ctx.Done():
		// 其他RPC调用调用失败
		return nil, ctx.Err()
	case err := <-errChan:
		// 本RPC调用失败，返回错误信息
		return nil, err
	case dataArr := <-resultChan:
		// 本RPC调用成功，不返回错误信息
		return dataArr, nil
	}
}

// reqData 请求当前集群指定key数据
func reqData(ctx context.Context, node *Node, in *ReqData) ([]byte, error) {
	var (
		resultChan = make(chan []byte)
		errChan    = make(chan error)
	)
	go func() {
		var (
			resp interface{}
			err  error
		)
		resp, err = gnomon.GRPCRequestPools(node.Url, func(conn *grpc.ClientConn) (interface{}, error) {
			// 创建grpc客户端
			cli := NewRaftClient(conn)
			//客户端向grpc服务端发起请求
			return cli.Data(context.Background(), in)
		})
		if nil != err {
			log.Warn("raft", log.Err(err))
			errChan <- err
		} else {
			resultChan <- resp.(*RespData).Value
		}
	}()
	select {
	case <-ctx.Done():
		// 其他RPC调用调用失败
		return nil, ctx.Err()
	case err := <-errChan:
		// 本RPC调用失败，返回错误信息
		return nil, err
	case bytes := <-resultChan:
		// 本RPC调用成功，不返回错误信息
		return bytes, nil
	}
}

// reqSyncData 同步数据
func reqSyncData(ctx context.Context, node *Node, in *ReqSyncData) error {
	var (
		resultChan = make(chan struct{})
		errChan    = make(chan error)
	)
	go func() {
		var (
			err error
		)
		//_, err = rpc(url, func(conn *grpc.ClientConn) (interface{}, error) {
		//	// 创建grpc客户端
		//	cli := NewRaftClient(conn)
		//	//客户端向grpc服务端发起请求
		//	return cli.SyncData(context.Background(), in)
		//})
		_, err = gnomon.GRPCRequestPools(node.Url, func(conn *grpc.ClientConn) (interface{}, error) {
			// 创建grpc客户端
			cli := NewRaftClient(conn)
			//客户端向grpc服务端发起请求
			return cli.SyncData(context.Background(), in)
		})
		if nil != err {
			log.Warn("raft", log.Err(err))
			errChan <- err
		} else {
			resultChan <- struct{}{}
		}
	}()
	select {
	case <-ctx.Done():
		// 其他RPC调用调用失败
		return ctx.Err()
	case err := <-errChan:
		// 本RPC调用失败，返回错误信息
		return err
	case <-resultChan:
		// 本RPC调用成功，不返回错误信息
		return nil
	}
}

// reqVote 发起选举，索要选票
func reqVote(ctx context.Context, node *Node, in *ReqVote) bool {
	var (
		resultChan = make(chan bool)
		errChan    = make(chan error)
	)
	go func() {
		var (
			resp interface{}
			err  error
		)
		resp, err = gnomon.GRPCRequestPools(node.Url, func(conn *grpc.ClientConn) (interface{}, error) {
			// 创建grpc客户端
			cli := NewRaftClient(conn)
			//客户端向grpc服务端发起请求
			return cli.Vote(context.Background(), in)
		})
		if nil != err {
			log.Warn("raft", log.Err(err))
			errChan <- err
		} else {
			resultChan <- resp.(*RespVote).VoteGranted
		}
	}()
	select {
	case <-ctx.Done():
		// 其他RPC调用调用失败
		return false
	case <-errChan:
		// 本RPC调用失败，返回错误信息
		return false
	case voteGranted := <-resultChan:
		// 本RPC调用成功，不返回错误信息
		return voteGranted
	}
}
