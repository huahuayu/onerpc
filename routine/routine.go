package routine

import (
	"github.com/huahuayu/onerpc/chainlist"
	"github.com/huahuayu/onerpc/flags"
	"github.com/huahuayu/onerpc/global"
	"github.com/huahuayu/onerpc/logger"
	"github.com/huahuayu/onerpc/rpc"
	"os"
	"sync"
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
	startTime := time.Now()
	var RPCMap map[int64]rpc.RPCs
	_, _, RPCMap, err := chainlist.GetAllChainInfo()
	if err != nil {
		return err
	}
	global.RPCMap = RPCMap

	fallbackMap := make(map[int64]rpc.RPCs)
	for chainID, rpcList := range flags.FallbackRPCs {
		rpcs := rpc.NewRPCs(chainID, rpcList)
		fallbackMap[chainID] = rpcs
	}
	global.FallbackMap = fallbackMap

	var (
		wg                sync.WaitGroup
		totalRPCs         int
		totalFallbackRPCs int
	)
	for _, rpcs := range RPCMap {
		totalRPCs += len(rpcs)
		wg.Add(1)
		go func(rpcs rpc.RPCs) {
			defer wg.Done()
			rpcs.RefreshRpcStatus()
		}(rpcs)
	}
	for _, rpcs := range fallbackMap {
		totalFallbackRPCs += len(rpcs)
		wg.Add(1)
		go func(rpcs rpc.RPCs) {
			defer wg.Done()
			rpcs.RefreshRpcStatus()
		}(rpcs)
	}
	wg.Wait()
	elapsedTime := time.Since(startTime)
	logger.Logger.Info().Msgf("%d chains with %d rpcs, %d fallback rpcs refreshed in %v", len(RPCMap), totalRPCs, totalFallbackRPCs, elapsedTime)
	return nil
}
