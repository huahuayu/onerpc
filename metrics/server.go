package metrics

import (
	"github.com/huahuayu/onerpc/flags"
	"github.com/huahuayu/onerpc/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

func StartServer() {
	port := *flags.MetricsPort
	logger.Logger.Info().Msg("starting metrics server on port " + port)
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":"+port, nil)
}
