package main

import (
	"github.com/huahuayu/onerpc/flags"
	"github.com/huahuayu/onerpc/gateway"
	"github.com/huahuayu/onerpc/logger"
	"github.com/huahuayu/onerpc/metrics"
	"github.com/huahuayu/onerpc/routine"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	flags.Init()
	logger.Init(*flags.LogLevel, *flags.LogCaller)
	gateway.Init()
	logger.Logger.Info().Msg("refreshing chain info...")
	routine.RefreshChainInfo()
	go gateway.StartGatewayServer()
	if *flags.Metrics {
		go metrics.StartServer()
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	<-shutdown
	logger.Logger.Info().Msg("shutting down the server...")
}
