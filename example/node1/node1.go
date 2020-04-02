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

package main

import (
	"github.com/aberic/gnomon"
	"github.com/aberic/gnomon/grope"
	"github.com/aberic/raft4go"
	"net/http"
)

func main() {
	node := &raft4go.Node{Id: "1", Url: "127.0.0.1:19877"}
	nodes := []*raft4go.Node{
		{Id: "2", Url: "127.0.0.1:19878"},
		//{Id: "3", Url: "127.0.0.1:19879"},
	}
	raft4go.RaftStartWithParams(&raft4go.Params{
		Node:          node,
		Nodes:         nodes,
		TimeHeartbeat: 0,
		TimeCheckReq:  0,
		TimeoutReq:    0,
		PortReq:       "19877",
		Log: &raft4go.Log{
			Dir:         "tmp/log",
			FileMaxSize: 1,
			FileMaxAge:  1,
			Utc:         false,
			Level:       "debug",
			Production:  false,
		},
	})

	httpServe := gnomon.Grope().NewHttpServe()
	router(httpServe)
	gnomon.Grope().ListenAndServe(":8080", httpServe)
}

func router(hs *grope.GHttpServe) {
	// 仓库相关路由设置
	route := hs.Group("/raft")
	route.Get("/status", nil, status)
	route.Get("/put/:key/:value", nil, put)
	route.Get("/get/:key", nil, get)
	route.Get("/node/list", nil, nodeList)
}

func status(_ http.ResponseWriter, _ *http.Request, _ interface{}, _ map[string]string) (respModel interface{}, custom bool) {
	return raft4go.Status(), false
}

func put(_ http.ResponseWriter, _ *http.Request, _ interface{}, paramMaps map[string]string) (respModel interface{}, custom bool) {
	key := paramMaps["key"]
	value := paramMaps["value"]
	gnomon.Log().Info("raft", gnomon.Log().Field("key", key),
		gnomon.Log().Field("value", value))
	return raft4go.Put(key, []byte(value)), false
}

func get(_ http.ResponseWriter, _ *http.Request, _ interface{}, paramMaps map[string]string) (respModel interface{}, custom bool) {
	key := paramMaps["key"]
	gnomon.Log().Info("raft", gnomon.Log().Field("key", key))
	if bytes, err := raft4go.Get(key); nil != err {
		return err, false
	} else {
		return string(bytes), false
	}
}

func nodeList(_ http.ResponseWriter, _ *http.Request, _ interface{}, _ map[string]string) (respModel interface{}, custom bool) {
	return raft4go.NodeList(), false
}
