package global

import (
	"github.com/huahuayu/onerpc/rpc"
)

var (
	RPCMap      map[int64]rpc.RPCs
	FallbackMap map[int64]rpc.RPCs
)
