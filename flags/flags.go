package flags

import (
	"encoding/json"
	"flag"
	"github.com/joho/godotenv"
	"log"
	"os"
	"strings"
)

type RPC struct {
	ChainID int64    `json:"chainID"`
	RPC     []string `json:"rpc"`
}

type RPCGroup []*RPC // e.g. `[{"chainID":1,"rpc":["https://eth.llamarpc.com","https://rpc.builder0x69.io"]}]`

func (r RPCGroup) ToRPCMap() map[int64][]string {
	rpcMap := make(map[int64][]string)
	for _, rpc := range r {
		rpcMap[rpc.ChainID] = rpc.RPC
	}
	return rpcMap
}

var (
	// Flags that can be set by either env variables or flags
	Port                 = flag.String("port", "", "RPC gateway port, e.g. 8080")
	Metrics              = flag.Bool("metrics", false, "Enable prometheus metrics")
	MetricsPort          = flag.String("metricsPort", "", "Metrics server port")
	rpcs                 = flag.String("rpcs", "", "Additional rpcs besides the public ones, e.g. [{\"chainID\":1,\"rpc\":[\"https://eth.llamarpc.com\",\"https://rpc.builder0x69.io\"]}]")
	fallbacks            = flag.String("fallback", "", "Fallback rpcs, e.g. [{\"chainID\":1,\"rpc\":[\"https://eth.llamarpc.com\",\"https://rpc.builder0x69.io\"]}]")
	EnableRateLimit      = flag.Bool("enableRateLimit", false, "Enable rate limit")
	RateLimitWithoutAuth = flag.Int("rateLimitWithoutAuth", 100, "Rate limit per second without auth")
	RateLimitWithAuth    = flag.Int("rateLimitWithAuth", 0, "Rate limit per second with auth (0: no limit)")

	// Flags with default values
	Replica                = flag.Int("replica", 1, "replica rpcs to send request")
	cacheableMethods       = flag.String("cacheableMethods", "eth_getTransactionByHash,eth_getBlockByNumber,eth_getTransactionReceipt,eth_getBlockReceipts,eth_getTransactionByBlockHashAndIndex,eth_getTransactionByBlockNumberAndIndex,eth_getBlockByHash,eth_getBlockTransactionCountByHash,eth_getBlockTransactionCountByNumber", "Cacheable methods")
	CacheTTL               = flag.Uint("cache_ttl", 60, "Cache TTL in minutes")
	LogLevel               = flag.Int("logLevel", 1, "Log level, -1: trace, 0: debug, 1: info, 2: warn, 3: error, 4: fatal, 5: panic")
	LogCaller              = flag.Bool("logCaller", false, "Log caller")
	RPCTimeout             = flag.Int("rpcTimeout", 20, "RPC timeout in seconds")
	RPCHealthCheckInterval = flag.Int("rpcHealthCheckInterval", 1, "RPC health check interval in minutes")

	// Transformed flags for easier use
	AdditionalRPCs   = make(map[int64][]string)
	FallbackRPCs     = make(map[int64][]string)
	CacheableMethods = make(map[string]bool)
)

func Init() {
	// Load .env file
	err := godotenv.Overload()
	if err != nil {
		if os.IsNotExist(err) {
			// If the .env file does not exist, log a warning and continue
			log.Println(".env file does not exist, skipping .env file loading.")
		} else {
			log.Fatalf("error loading .env file: " + err.Error())
		}
	}
	flag.Parse()
	// Check if required flags are provided
	if *Port == "" {
		*Port = os.Getenv("GATEWAY_PORT")
		if *Port == "" {
			log.Fatalf("port is required")
		}
	}

	// Parse metrics flag
	if *Metrics == false {
		*Metrics = strings.ToLower(os.Getenv("METRICS")) == "true"
	}

	if *Metrics {
		if *MetricsPort == "" {
			*MetricsPort = os.Getenv("METRICS_PORT")
			if *MetricsPort == "" {
				log.Fatalf("metricsPort is required if metrics is enabled")
			}
		}
	}

	// Parse replica flag
	if *Replica <= 0 {
		log.Fatalf("replica should be greater than 0")
	}

	var additionalRPCGroup RPCGroup
	if *rpcs != "" {
		err := json.Unmarshal([]byte(*rpcs), &additionalRPCGroup)
		if err != nil {
			log.Fatalf("failed to parse rpcs flag: %v", err)
		}
	} else {
		rpcEnv := os.Getenv("RPCS")
		if rpcEnv != "" {
			err := json.Unmarshal([]byte(rpcEnv), &additionalRPCGroup)
			if err != nil {
				log.Fatalf("failed to parse RPCS env variable: %v", err)
			}
		}
	}
	for _, rpc := range additionalRPCGroup {
		AdditionalRPCs[rpc.ChainID] = rpc.RPC
	}

	var fallbackRPCGroup RPCGroup
	if *fallbacks != "" {
		err := json.Unmarshal([]byte(*rpcs), &fallbackRPCGroup)
		if err != nil {
			log.Fatalf("failed to parse rpcs flag: %v", err)
		}
	} else {
		rpcEnv := os.Getenv("FALLBACKS")
		if rpcEnv != "" {
			err := json.Unmarshal([]byte(rpcEnv), &fallbackRPCGroup)
			if err != nil {
				log.Fatalf("failed to parse FALLBACKS env variable: %v", err)
			}
		}
	}
	for _, rpc := range fallbackRPCGroup {
		FallbackRPCs[rpc.ChainID] = rpc.RPC
	}

	if *EnableRateLimit == false {
		*EnableRateLimit = strings.ToLower(os.Getenv("ENABLE_RATE_LIMIT")) == "true"
	}

	// Parse cacheable methods
	*cacheableMethods = strings.ReplaceAll(*cacheableMethods, " ", "")
	methods := strings.Split(*cacheableMethods, ",")
	for _, method := range methods {
		CacheableMethods[method] = true
	}
}
