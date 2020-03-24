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
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"net"
	"strconv"
	"strings"
)

// rpc 通过rpc进行通信 protoc --go_out=plugins=grpc:. grpc/proto/*.proto
func rpc(url string, business func(conn *grpc.ClientConn) (interface{}, error)) (interface{}, error) {
	var (
		conn *grpc.ClientConn
		err  error
	)
	// 创建一个grpc连接器
	if conn, err = grpc.Dial(url, grpc.WithInsecure()); nil != err {
		return nil, err
	}
	// 请求完毕后关闭连接
	defer func() { _ = conn.Close() }()
	return business(conn)
}

// getGRPCClientIP 取出gRPC客户端的ip地址和端口号
//
// string form of address (for example, "192.0.2.1:25", "[2001:db8::1]:80")
func getGRPCClientIP(ctx context.Context) (address string, port int, err error) {
	var (
		pr *peer.Peer
		ok bool
	)
	if pr, ok = peer.FromContext(ctx); !ok {
		err = fmt.Errorf("[getGRPCClientIP] invoke FromContext() failed")
		return
	}
	if pr.Addr == net.Addr(nil) {
		err = fmt.Errorf("[getGRPCClientIP] peer.Addr is nil")
		return
	}
	addSlice := strings.Split(pr.Addr.String(), ":")
	address = addSlice[0]
	if port, err = strconv.Atoi(addSlice[1]); nil != err {
		err = fmt.Errorf("[getGRPCClientIP] peer.Addr.port parse int error, err: %s", err.Error())
		return
	}
	return
}
