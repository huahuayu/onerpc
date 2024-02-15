package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	CallsCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rpc_forward_calls_total",
			Help: "Total number of calls to each URL",
		},
		[]string{"chainID", "url"}, // labels
	)

	CallDurationHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rpc_forward_call_duration_seconds",
			Help:    "Duration of calls to each URL",
			Buckets: prometheus.DefBuckets, // Default buckets, can be customized
		},
		[]string{"chainID", "url"},
	)

	CallErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rpc_call_errors_total",
			Help: "Total number of errors occurred in rpc call",
		},
		[]string{"chainID", "url", "error"},
	)

	LatestBlockHeightGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rpc_latest_block_height",
			Help: "Latest block height of each URL",
		},
		[]string{"chainID", "url"},
	)
)
