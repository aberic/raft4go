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
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"sync"
)

var (
	connections = map[string]*grpc.ClientConn{}
	clients     = map[string]RaftClient{}
	muConn      sync.Mutex
	muClient    sync.Mutex
)

func getRaftClient(url string) RaftClient {
	if client, ok := clients[url]; ok {
		if conn, exist := connections[url]; exist && conn.GetState() != connectivity.Shutdown && conn.GetState() != connectivity.TransientFailure {
			return client
		}
	}
	return getRPCClient(url)
}

func getRPCClient(url string) RaftClient {
	// 创建grpc客户端
	defer muClient.Unlock()
	client := NewRaftClient(getRPCConn(url))
	muClient.Lock()
	clients[url] = client
	return client
}

func getRPCConn(url string) *grpc.ClientConn {
	if conn, ok := connections[url]; ok && conn.GetState() != connectivity.Shutdown && conn.GetState() != connectivity.TransientFailure {
		return conn
	}
	defer muConn.Unlock()
	muConn.Lock()
	// 创建一个grpc连接器
	if conn, err := grpc.Dial(url, grpc.WithInsecure()); nil != err {
		panic(err)
	} else {
		connections[url] = conn
		return conn
	}
}
