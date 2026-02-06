package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	rpcRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rpc_requests_total",
			Help: "Total RPC requests",
		},
		[]string{"service", "method", "code"},
	)
	rpcRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rpc_request_duration_seconds",
			Help:    "RPC request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method"},
	)
)

func init() {
	prometheus.MustRegister(rpcRequestsTotal, rpcRequestDurationSeconds)
}
