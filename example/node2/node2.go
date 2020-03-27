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
	"github.com/aberic/raft4go"
	"log"
	"net/http"
	"time"
)

type myHandler struct{}

func (*myHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("this is version 3"))
}

func sayHello(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hello"))
}

func sayBye(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {

	}
	// 睡眠4秒  上面配置了3秒写超时，所以访问 “/bye“路由会出现没有响应的现象
	time.Sleep(4 * time.Second)
	w.Write([]byte("bye bye ,this is v3 httpServer"))
}

func main() {
	node := &raft4go.Node{Id: "2", Url: "127.0.0.1:19878"}
	nodes := []*raft4go.Node{
		{Id: "1", Url: "127.0.0.1:19877"},
		{Id: "3", Url: "127.0.0.1:19879"},
	}
	raft4go.RaftStartWithParams(&raft4go.Params{
		Node:         node,
		Nodes:        nodes,
		TimeCheckReq: 0,
		TimeoutReq:   0,
		PortReq:      "19878",
		Log: &raft4go.Log{
			Dir:         "tmp/log",
			FileMaxSize: 1,
			FileMaxAge:  1,
			Utc:         true,
			Level:       "debug",
			Production:  false,
		},
	})

	mux := http.NewServeMux()
	mux.Handle("/", &myHandler{})
	mux.HandleFunc("/hi", sayHello)
	mux.HandleFunc("/bye", sayBye)

	server := &http.Server{
		Addr:         ":8081",
		WriteTimeout: time.Second * 3, //设置3秒的写超时
		Handler:      mux,
	}
	log.Fatal(server.ListenAndServe())
}
