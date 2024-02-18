package routine

import (
	"github.com/huahuayu/onerpc/chainlist"
	"github.com/huahuayu/onerpc/flags"
	"github.com/huahuayu/onerpc/global"
	"github.com/huahuayu/onerpc/logger"
	"github.com/huahuayu/onerpc/rpc"
	"os"
	"time"
)

func RefreshChainInfo() {
	// Call updateChainInfo immediately for the first time
	err := updateChainInfo()
	if err != nil {
		logger.Logger.Error().Str("error", err.Error()).Msg("init chainInfo")
		os.Exit(-1)
	}

	// Create a ticker that update the chain info every 1 hour
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for {
			select {
			case <-ticker.C:
				updateChainInfo()
			}
		}
	}()
}

func updateChainInfo() error {
	var RPCMap map[int64]rpc.RPCs
	_, _, RPCMap, err := chainlist.GetAllChainInfo()
	if err != nil {
		return err
	}
	oldRPCMap := global.RPCMap
	global.RPCMap = RPCMap

	fallbackMap := make(map[int64]rpc.RPCs)
	for chainID, rpcList := range flags.FallbackRPCs {
		rpcs := rpc.NewRPCs(chainID, rpcList)
		fallbackMap[chainID] = rpcs
	}
	oldFallbackMap := global.FallbackMap
	global.FallbackMap = fallbackMap

	// Stop the old rpcs refresh routines
	for _, rpcs := range oldRPCMap {
		if rpcs != nil {
			rpcs.StopRefreshRpcStatus()
		}
	}
	for _, rpcs := range oldFallbackMap {
		if rpcs != nil {
			rpcs.StopRefreshRpcStatus()
		}
	}

	var (
		totalRPCs         int
		totalFallbackRPCs int
	)
	for _, rpcs := range RPCMap {
		totalRPCs += len(rpcs)
		go func(rpcs rpc.RPCs) {
			rpcs.RefreshRpcStatus()
		}(rpcs)
	}
	for _, rpcs := range fallbackMap {
		totalFallbackRPCs += len(rpcs)
		go func(rpcs rpc.RPCs) {
			rpcs.RefreshRpcStatus()
		}(rpcs)
	}
	logger.Logger.Info().Msgf("%d chains with %d rpcs, and %d fallback rpcs refreshed", len(RPCMap), totalRPCs, totalFallbackRPCs)
	return nil
}
