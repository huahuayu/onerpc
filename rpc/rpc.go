package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/huahuayu/onerpc/flags"
	"github.com/huahuayu/onerpc/logger"
	"github.com/huahuayu/onerpc/metrics"
	"io"
	"math/big"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"sync"
	"time"
)

type RPC struct {
	ChainID int64
	URL     string
	Height  int64
	Status  Status
	mutex   sync.Mutex
	client  *http.Client
}

type RPCs []*RPC

type Status string

const (
	Unknown Status = "Unknown"
	OK      Status = "OK"
	Down    Status = "Down"
)

func NewRPC(chainID int64, url string) *RPC {
	return &RPC{
		ChainID: chainID,
		URL:     url,
		Height:  0,
		Status:  Unknown,
		mutex:   sync.Mutex{},
		client: &http.Client{
			Timeout: time.Duration(*flags.RPCTimeout) * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:       10,
				IdleConnTimeout:    30 * time.Second,
				DisableCompression: true,
			},
		},
	}
}

func (r *RPC) updateHeight() error {
	payload := []byte(`{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`)
	resp, err := r.client.Post(r.URL, "application/json", bytes.NewBuffer(payload))
	if err != nil || resp == nil || resp.StatusCode != http.StatusOK {
		r.mutex.Lock()
		r.Status = Down
		r.mutex.Unlock()
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	type JSONRPCResponse struct {
		JSONRPC string `json:"jsonrpc"`
		Id      int    `json:"id"`
		Result  any    `json:"result"`
	}

	var response JSONRPCResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return fmt.Errorf("unmarshal response err: %s, url: %s", err, r.URL)
	}

	number, ok := response.Result.(string)
	if !ok || number == "" || number == "0x" || number == "0x0" {
		return fmt.Errorf("invalid block number: %s, url: %s", number, r.URL)
	}
	blockNumber, ok := new(big.Int).SetString(number[2:], 16) // Remove the "0x" prefix and convert from hex to int
	if !ok {
		return nil
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.Status = OK
	r.Height = blockNumber.Int64()
	logger.Logger.Debug().
		Str("chainID", strconv.FormatInt(r.ChainID, 10)).
		Str("url", r.URL).
		Int64("height", r.Height).
		Msg("updateHeight")
	// Set the gauge to the latest block height
	metrics.LatestBlockHeightGauge.WithLabelValues(fmt.Sprint(r.ChainID), r.URL).Set(float64(r.Height))

	return nil
}

func (r *RPC) forward(requestBody []byte, httpProxy ...string) ([]byte, error) {
	var client = r.client

	if len(httpProxy) > 0 && httpProxy[0] != "" {
		proxyURL, err := url.Parse(httpProxy[0])
		if err != nil {
			return nil, err
		}
		client = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			},
		}
	}

	// Start the timer
	startTime := time.Now()

	resp, err := client.Post(r.URL, "application/json", bytes.NewBuffer(requestBody))

	// Stop the timer
	duration := time.Since(startTime).Seconds()

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Check if the response is an error
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("response status code: %d, url: %s, body: %s", resp.StatusCode, r.URL, string(body))
	}

	// Check if body contains rate limit error
	if bytes.Contains(body, []byte("rate")) {
		return nil, fmt.Errorf("rate limit error: %s, url: %s", string(body), r.URL)
	}

	// Check if body is valid JSON
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, fmt.Errorf("unmarshal response err: %s, url: %s", err, r.URL)
	}

	// Log the metrics
	metrics.CallDurationHistogram.WithLabelValues(fmt.Sprint(r.ChainID), r.URL).Observe(duration)
	metrics.CallsCounter.WithLabelValues(fmt.Sprint(r.ChainID), r.URL).Inc()

	return body, nil
}

func NewRPCs(chainID int64, urls []string) RPCs {
	if len(urls) == 0 {
		return nil
	}
	rpcs := make(RPCs, 0)
	for _, url := range urls {
		rpcs = append(rpcs, NewRPC(chainID, url))
	}
	return rpcs
}

// GetRandomRPC returns a random RPC from the list of RPCs, which the status is OK and the height is the highest as possible
func (rpcs RPCs) GetRandomRPC(num int, exclude RPCs) RPCs {
	// Filter RPCs that might work
	mightWorkRPCs := make(RPCs, 0)
	for _, rpc := range rpcs {
		if rpc.Status == OK {
			mightWorkRPCs = append(mightWorkRPCs, rpc)
		}
	}

	// Exclude the RPCs that are in the exclude list
	for _, rpc := range exclude {
		for i, mightWorkRPC := range mightWorkRPCs {
			if rpc.URL == mightWorkRPC.URL {
				mightWorkRPCs = append(mightWorkRPCs[:i], mightWorkRPCs[i+1:]...)
				break
			}
		}
	}

	if len(mightWorkRPCs) == 0 {
		return nil
	}

	// Sort the RPCs by height in descending order
	sort.Slice(mightWorkRPCs, func(i, j int) bool {
		return mightWorkRPCs[i].Height > mightWorkRPCs[j].Height
	})

	// Group the RPCs with the same height together
	groupedRPCs := make(map[int64]RPCs)
	for _, rpc := range mightWorkRPCs {
		groupedRPCs[rpc.Height] = append(groupedRPCs[rpc.Height], rpc)
	}

	// Sort the heights in descending order
	heights := make([]int64, 0, len(groupedRPCs))
	for height := range groupedRPCs {
		heights = append(heights, height)
	}
	sort.Slice(heights, func(i, j int) bool {
		return heights[i] > heights[j]
	})

	// Start from the group with the highest height, select RPCs randomly until the number of RPCs is satisfied
	selectedRPCs := make(RPCs, 0, num)
	for _, height := range heights {
		sameHeightRPCs := groupedRPCs[height]
		perm := rand.Perm(len(sameHeightRPCs))
		for _, idx := range perm {
			if len(selectedRPCs) < num {
				selectedRPCs = append(selectedRPCs, sameHeightRPCs[idx])
			} else {
				return selectedRPCs
			}
		}
	}

	return selectedRPCs
}

func (rpcs RPCs) RefreshRpcStatus() {
	wg := sync.WaitGroup{}
	for _, rpc := range rpcs {
		wg.Add(1)
		go func(rpc *RPC) {
			defer wg.Done()
			rpc.updateHeight()
		}(rpc)
	}
	wg.Wait()

	// Start a goroutine to update the rpc status every 1 min
	ticker := time.NewTicker(time.Duration(*flags.RPCHealthCheckInterval) * time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				for _, rpc := range rpcs {
					go rpc.updateHeight()
				}
			}
		}
	}()
}

// SendRequest sends JSON RPC request by selecting a random RPC from the list of good status RPCs
func (rpcs RPCs) SendRequest(body []byte, numberOfRPCs int, exclude RPCs, httpProxy ...string) (response []byte, origins RPCs, err error) {
	// Select a random RPC from the list of RPCs
	randRPCs := rpcs.GetRandomRPC(numberOfRPCs, exclude)
	if len(randRPCs) == 0 {
		return nil, nil, fmt.Errorf("no node available")
	}

	// Send the request to randRPCs concurrently, but only use the first response that is not an error
	errChan := make(chan error, len(randRPCs))
	respChan := make(chan []byte, len(randRPCs))
	for _, rpc := range randRPCs {
		go func(rpc *RPC) {
			resp, err := rpc.forward(body, httpProxy...)
			if err != nil {
				metrics.CallErrorCounter.WithLabelValues(fmt.Sprint(rpc.ChainID), rpc.URL, err.Error()).Inc()
				errChan <- err
				return
			}
			respChan <- resp
		}(rpc)
	}

	// Wait for the first response, if there is an error, wait for the next response, and so on, if all responses are errors, return an error
	for i := 0; i < len(randRPCs); i++ {
		select {
		case err := <-errChan:
			if err != nil {
				continue
			}
		case resp := <-respChan:
			return resp, randRPCs, nil
		}
	}

	return nil, randRPCs, fmt.Errorf("no response received")
}

func (rpcs RPCs) contains(rpc string) bool {
	for _, r := range rpcs {
		if r.URL == rpc {
			return true
		}
	}
	return false
}
