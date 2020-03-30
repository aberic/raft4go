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

type RoleStatus int

const (
	RoleStatusLeader RoleStatus = iota
	RoleStatusCandidate
	RoleStatusFollower
)

// role 角色转换接口
type role interface {
	// work 开始本职工作
	start()
	// update 更新当前状态
	update(hb *heartBeat)
	// release 角色释放
	release()
	// put 角色所属集群新增数据
	put(key string, value []byte) error
	// syncData 请求同步数据
	syncData(req *ReqSyncData) error
	// vote 接收请求投票数据
	vote(req *ReqVote) (bool, error)
	// roleStatus 获取角色状态，0-leader、1-candidate、2-follower
	roleStatus() RoleStatus
}

type roleBase struct {
	status RoleStatus
}

func (rb *roleBase) setStatus(status RoleStatus) {
	rb.status = status
}

func (rb *roleBase) roleStatus() RoleStatus {
	return rb.status
}

type heartBeat struct {
	ReqHeartBeat
	clientAddress string
	clientPort    int
}
