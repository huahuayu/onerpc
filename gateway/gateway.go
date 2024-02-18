package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/huahuayu/onerpc/cache"
	"github.com/huahuayu/onerpc/flags"
	"github.com/huahuayu/onerpc/global"
	"github.com/huahuayu/onerpc/logger"
	"github.com/huahuayu/onerpc/rpc"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	io.Writer
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	return rw.Writer.Write(b)
}

var validApiKeys = make(map[string]bool)

func generateAndStoreAPIKeys() {
	for i := 0; i < 10; i++ {
		randomUUID := uuid.New().String()
		fullApiKey := strings.ReplaceAll(randomUUID, "-", "")
		shortApiKey := fullApiKey[:16] // Take first 16 characters
		validApiKeys[shortApiKey] = true
	}
	// Create a file to store the API keys, if the directory does not exist, create it
	err := os.MkdirAll("./apikey", os.ModePerm)
	if err != nil {
		logger.Logger.Error().Msgf("Error creating apikey directory: %s", err)
		return
	}
	apiKeyFile, err := os.Create("./apikey/keys.txt")
	if err != nil {
		logger.Logger.Error().Msgf("Error creating apikey.txt: %s", err)
		return
	}
	defer apiKeyFile.Close()
	for apiKey := range validApiKeys {
		_, err := apiKeyFile.WriteString(apiKey + "\n")
		if err != nil {
			logger.Logger.Error().Msgf("Error writing to apikey.txt: %s", err)
			return
		}
	}
	logger.Logger.Info().Any("keys", validApiKeys).Msg("API keys generated")
}

// Create a cache with a clean tick of 1 minutes
var (
	responseCache  cache.ICache[string, []byte]
	rateLimitCache cache.ICache[string, int]
)

func Init() {
	responseCache = cache.New[string, []byte](time.Duration(*flags.CacheTTL) * time.Minute)
	rateLimitCache = cache.New[string, int](1 * time.Second)
}

// Handler for /chain/ endpoint
func chainHandler(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")

	// Ensure there's enough parts in the path for the chainID
	if len(pathParts) < 3 {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	chainId, err := strconv.ParseInt(pathParts[2], 10, 64)
	if err != nil {
		logger.Logger.Error().Msgf("Invalid chainID: %s", err)
		http.Error(w, "Invalid chainID", http.StatusBadRequest)
		return
	}

	rpcs, ok := global.RPCMap[chainId]
	if !ok {
		logger.Logger.Error().Msgf("No node found for the given chainID: %s", chainId)
		http.Error(w, "No node found for the given chainID", http.StatusNotFound)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	// Check if body is a valid JSON
	var jsonBody map[string]interface{}
	if err := json.Unmarshal(body, &jsonBody); err != nil {
		http.Error(w, "Invalid JSONRPC request", http.StatusBadRequest)
		return
	}
	response, origins, err := rpcs.SendRequest(body, *flags.Replica, nil)
	if err != nil {
		// Retry if the first request failed, exclude previous origins
		var secondOrigins rpc.RPCs
		response, secondOrigins, err = rpcs.SendRequest(body, 1, origins)
		if err != nil {
			// Retry if the second request failed, exclude both previous origins
			exclude := append(origins, secondOrigins...)
			response, _, err = rpcs.SendRequest(body, 1, exclude)
			if err != nil {
				// Use fallback node if all retry failed
				fallbackRPCs, _ := global.FallbackMap[chainId]
				if fallbackRPCs != nil {
					response, _, err = fallbackRPCs.SendRequest(body, 1, nil)
				}
				if err != nil {
					logger.Logger.Error().Msgf("Error sending request: %s", err)
					http.Error(w, "Error sending request: "+err.Error(), http.StatusTooManyRequests)
					return
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func StartGatewayServer() {
	http.HandleFunc("/chain/", loggerMiddleware(authMiddleware(cacheMiddleware(chainHandler))))
	port := *flags.Port
	if *flags.EnableRateLimit {
		generateAndStoreAPIKeys()
	}
	logger.Logger.Info().Msgf("Starting gateway server on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		panic("Failed to start server: " + err.Error())
	}
}

func loggerMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now() // Record the start time

		// Read the request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Logger.Error().Msgf("Error reading request body: %s", err)
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}
		// Create a new ReadCloser for the request body
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		// Generate a new UUID for the request
		requestID := uuid.New()

		// Add the request ID to the context
		ctx := context.WithValue(r.Context(), "requestID", requestID)
		r = r.WithContext(ctx)

		// Extract the method & params from the request
		var rpcRequest struct {
			Method string        `json:"method"`
			Params []interface{} `json:"params"`
		}
		if err := json.NewDecoder(bytes.NewBuffer(body)).Decode(&rpcRequest); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// Add method to the context
		if rpcRequest.Method != "" {
			ctx = context.WithValue(r.Context(), "method", rpcRequest.Method)
			r = r.WithContext(ctx)
		}

		// Add params to the context
		bs, err := json.Marshal(rpcRequest.Params)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		params := string(bs)
		if params != "" {
			ctx = context.WithValue(r.Context(), "params", params)
			r = r.WithContext(ctx)
		}

		// Extract the IP address from the request
		ip := getIPAddress(r)

		// Create a buffer to capture the response
		var buffer bytes.Buffer

		// Create a multiwriter that writes to both the original http.ResponseWriter and the buffer
		multiWriter := io.MultiWriter(w, &buffer)

		// Replace the http.ResponseWriter with our multiwriter
		w = &responseWriter{ResponseWriter: w, Writer: multiWriter}

		// Call the actual handler
		next.ServeHTTP(w, r)

		// Calculate the duration
		duration := time.Since(start)

		// Extract logging info from the context
		var method string
		methodCtx := r.Context().Value("method")
		if methodCtx != nil {
			method = methodCtx.(string)
		}

		// Log the request and response info
		logger.Logger.Info().
			Str("requestID", requestID.String()).
			Str("ip", ip).
			Str("chainID", strings.Split(r.URL.Path, "/")[2]).
			Str("method", method).
			Str("timeUsed", duration.String()).
			Msg("reqInfo")

		logger.Logger.Debug().
			Str("requestID", requestID.String()).
			Str("request", string(body)).
			Msg("request")

		logger.Logger.Debug().
			Str("requestID", requestID.String()).
			Str("response", buffer.String()).
			Msg("response")
	}
}

func cacheMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get requestID from the context
		requestIDCtx := r.Context().Value("requestID")
		if requestIDCtx == nil {
			http.Error(w, "requestID not found", http.StatusBadRequest)
			return
		}
		requestID := requestIDCtx.(uuid.UUID)

		// Get method from the context
		methodCtx := r.Context().Value("method")
		if methodCtx == nil {
			http.Error(w, "method not found", http.StatusBadRequest)
			return
		}
		method := methodCtx.(string)

		// Check if the method should be cached
		if !flags.CacheableMethods[method] {
			next.ServeHTTP(w, r)
			return
		}

		// Get params from the context
		paramsCtx := r.Context().Value("params")
		if paramsCtx == nil {
			http.Error(w, "params not found", http.StatusBadRequest)
			return
		}
		params := paramsCtx.(string)

		// Generate a cache key using the method and parameters
		cacheKey := r.URL.String() + "-" + method + "-" + params

		// Check if the request has a Cache-Control header
		cacheControl := r.Header.Get("Cache-Control")

		// Try to get the response from the cache
		cachedResponse, found := responseCache.Get(cacheKey)
		if found && cacheControl != "no-cache" {
			logger.Logger.Debug().
				Str("requestID", requestID.String()).
				Str("chainID", strings.Split(r.URL.Path, "/")[2]).
				Str("method", method).
				Msg("cacheHit")
			w.Header().Set("Content-Type", "application/json")
			w.Write(cachedResponse)
			return
		}

		// Create a buffer to capture the response
		var buffer bytes.Buffer

		// Create a multiwriter that writes to both the original http.ResponseWriter and the buffer
		multiWriter := io.MultiWriter(w, &buffer)

		// Replace the http.ResponseWriter with our multiwriter
		w = &responseWriter{ResponseWriter: w, Writer: multiWriter}

		// Call the next middleware function
		next.ServeHTTP(w, r)

		// Verify if the response is valid JSON & result is not null
		var result map[string]interface{}
		err := json.Unmarshal(buffer.Bytes(), &result)
		if err != nil {
			return
		}
		if result["result"] == nil || result["result"] == "" || result["result"] == "null" {
			return
		}

		// Store the response in the cache
		responseCache.Set(cacheKey, buffer.Bytes(), time.Duration(*flags.CacheTTL)*time.Minute)
	}
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !*flags.EnableRateLimit {
			next.ServeHTTP(w, r)
			return
		}
		pathParts := strings.Split(r.URL.Path, "/")

		var apiKey string
		var isApiKeyValid bool

		// Check if an API key is provided
		if len(pathParts) >= 4 {
			apiKey = pathParts[3]
			isApiKeyValid = validateApiKey(apiKey)

			// If an API key is provided but not valid, deny the request
			if !isApiKeyValid {
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}
		}

		// Define rate limits
		var rateLimit int
		if isApiKeyValid {
			rateLimit = *flags.RateLimitWithAuth
			if rateLimit == 0 {
				rateLimit = int(^uint(0) >> 1)
			}
		} else {
			rateLimit = *flags.RateLimitWithoutAuth // rate limit for no API key
		}

		// Check rate limit for the IP or API key
		visitorKey := getRateLimitKey(r, apiKey, isApiKeyValid)
		count, found := rateLimitCache.Get(visitorKey)
		if found && count >= rateLimit {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		if !found {
			rateLimitCache.Set(visitorKey, 1, 1*time.Second) // Set the TTL for each count
		} else {
			rateLimitCache.Set(visitorKey, count+1, 1*time.Second) // Update the count and reset the TTL
		}

		// Proceed to the next handler if rate limit is not exceeded
		next.ServeHTTP(w, r)
	}
}

// Validate if the provided API key is valid (dummy implementation, replace with your actual validation logic)
func validateApiKey(apiKey string) bool {
	_, exists := validApiKeys[apiKey]
	return exists
}

// Get a unique key for the visitor to track rate limit
func getRateLimitKey(r *http.Request, apiKey string, isApiKeyValid bool) string {
	if isApiKeyValid {
		return "apikey:" + apiKey
	}
	return "ip:" + getIPAddress(r)
}

func getIPAddress(r *http.Request) string {
	// Try to get the real IP from the X-Forwarded-For header
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// X-Forwarded-For can contain multiple IPs, the first one is the original client
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 && ips[0] != "" {
			return strings.TrimSpace(ips[0])
		}
	}

	// Try to get the real IP from the X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// If no proxy is used, or the real IP is not in the headers, use RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr // In case of an error, return the full value
	}
	return ip
}
