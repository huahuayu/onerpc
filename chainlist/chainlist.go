package chainlist

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/huahuayu/onerpc/flags"
	"github.com/huahuayu/onerpc/rpc"
	"io"
	"math/big"
	"net/http"
	"sort"
	"strings"
)

// ChainList is the http get response from https://chainid.network/chains.json
type ChainList []*ChainInfo

type ChainInfo struct {
	Name             string            `json:"name"`
	Chain            string            `json:"chain"`
	Icon             string            `json:"icon,omitempty"`
	Tvl              float64           `json:"tvl,omitempty"`
	LlamaChainHeader *LlamaChainHeader `json:"LlamaChainHeader,omitempty"`
	LlamaChainDetail *LlamaChain       `json:"LlamaChainDetail,omitempty"`
	RPC              []string          `json:"rpc"`
	Features         []struct {
		Name string `json:"name"`
	} `json:"features,omitempty"`
	Faucets        []interface{} `json:"faucets"`
	NativeCurrency struct {
		Name     string `json:"name"`
		Symbol   string `json:"symbol"`
		Decimals int    `json:"decimals"`
	} `json:"nativeCurrency"`
	InfoURL   string `json:"infoURL"`
	ShortName string `json:"shortName"`
	ChainID   int64  `json:"chainId"`
	NetworkID int64  `json:"networkId"`
	Slip44    int    `json:"slip44,omitempty"`
	Ens       struct {
		Registry string `json:"registry"`
	} `json:"ens,omitempty"`
	Explorers []struct {
		Name     string `json:"name"`
		URL      string `json:"url"`
		Standard string `json:"standard"`
		Icon     string `json:"icon,omitempty"`
	} `json:"explorers,omitempty"`
	Title    string   `json:"title,omitempty"`
	Status   string   `json:"status,omitempty"`
	RedFlags []string `json:"redFlags,omitempty"`
	Parent   struct {
		Type    string `json:"type"`
		Chain   string `json:"chain"`
		Bridges []struct {
			URL string `json:"url"`
		} `json:"bridges"`
	} `json:"parent,omitempty"`
}

// LlamaChainHeaders is the http get response from https://api.llama.fi/v2/chains
type LlamaChainHeaders []*LlamaChainHeader

type LlamaChainHeader struct {
	GeckoID     any     `json:"gecko_id"` // maybe nil
	Tvl         float64 `json:"tvl"`
	TokenSymbol string  `json:"tokenSymbol"`
	CmcID       any     `json:"cmcId"` // maybe nil
	Name        string  `json:"name"`
	ChainID     any     `json:"chainId"` // maybe nil
}

// LlamaChains is the http get response from https://chainlist.org (after some processing)
type LlamaChains []*LlamaChain

type LlamaChain struct {
	Name  string `json:"name"`
	Chain string `json:"chain"`
	Icon  string `json:"icon,omitempty"`
	RPC   []struct {
		URL             string `json:"url"`
		Tracking        string `json:"tracking,omitempty"`
		TrackingDetails string `json:"trackingDetails,omitempty"`
		IsOpenSource    bool   `json:"isOpenSource,omitempty"`
	} `json:"rpc"`
	Features []struct {
		Name string `json:"name"`
	} `json:"features,omitempty"`
	Faucets        []interface{} `json:"faucets"`
	NativeCurrency struct {
		Name     string `json:"name"`
		Symbol   string `json:"symbol"`
		Decimals int    `json:"decimals"`
	} `json:"nativeCurrency"`
	InfoURL   string `json:"infoURL"`
	ShortName string `json:"shortName"`
	ChainID   int64  `json:"chainId"`
	NetworkID int64  `json:"networkId"`
	Slip44    int    `json:"slip44,omitempty"`
	Ens       struct {
		Registry string `json:"registry"`
	} `json:"ens,omitempty"`
	Explorers []struct {
		Name     string `json:"name"`
		URL      string `json:"url"`
		Standard string `json:"standard"`
		Icon     string `json:"icon,omitempty"`
	} `json:"explorers,omitempty"`
	Tvl       float64 `json:"tvl,omitempty"`
	ChainSlug string  `json:"chainSlug,omitempty"`
	Parent    struct {
		Type    string `json:"type"`
		Chain   string `json:"chain"`
		Bridges []struct {
			URL string `json:"url"`
		} `json:"bridges"`
	} `json:"parent,omitempty"`
	Status   string   `json:"status,omitempty"`
	Title    string   `json:"title,omitempty"`
	RedFlags []string `json:"redFlags,omitempty"`
}

// GetAllChainInfo gets all EVM chain info
// 1. Get all chain info from https://chainid.network/chains.json
// 2. Get all chain header info from https://api.llama.fi/v2/chains, and merge with the result from step 1, the key is chainID
// 3. Get all chain detail info from https://chainlist.org, and merge with the result from step 2, the key is chainID
// 4. I specially care about the rpc's url, so merge the LlamaChains.PageProps.Chain.RPC.URL with the ChainInfo.RPC, remove duplicated rpc url
// 5. Sort the result by Tvl in descending order
// GetAllChainInfo gets all EVM chain info
func GetAllChainInfo() (ChainList, map[int64]*ChainInfo, map[int64]rpc.RPCs, error) {
	// Step 1: Get all chain info from chainid.network
	chainList, err := getChainList()
	if err != nil {
		return nil, nil, nil, err
	}

	// Step 2: Get all chain header info from llama.fi and merge to chainList
	llamaChainHeaders, err := getLlamaChainHeaders()
	if err != nil {
		return nil, nil, nil, err
	}

	// Step 3: Get all chain info from chainlist.org and merge to chainList
	llamaChainDetails, err := getDetailLlamaChainInfo()
	if err != nil {
		return nil, nil, nil, err
	}

	// Step 4: Merge llamaChainHeaders, llamaChainDetails and private rpcs to chainList
	for _, chain := range chainList {
		if chainHeader, ok := llamaChainHeaders[chain.ChainID]; ok {
			chain.Tvl = chainHeader.Tvl
			chain.LlamaChainHeader = chainHeader
		}
		if chainDetail, ok := llamaChainDetails[chain.ChainID]; ok {
			chain.LlamaChainDetail = chainDetail
			mergeRpcInfo(chain, chainDetail, flags.AdditionalRPCs[chain.ChainID])
		}
	}

	// Exclude 1rpc.dev/* from the RPC list
	for _, chain := range chainList {
		for i, rpc := range chain.RPC {
			if strings.Contains(rpc, "1rpc.dev") {
				chain.RPC = append(chain.RPC[:i], chain.RPC[i+1:]...)
			}
		}
	}

	// Step 5: Sort the result by Tvl in descending order
	sort.Slice(chainList, func(i, j int) bool {
		if chainList[i].LlamaChainHeader == nil {
			return false
		}
		if chainList[j].LlamaChainHeader == nil {
			return true
		}
		return chainList[i].LlamaChainHeader.Tvl > chainList[j].LlamaChainHeader.Tvl
	})

	// Step 6: Convert chainList to a map for easy lookup
	chainMap := make(map[int64]*ChainInfo)
	for _, chain := range chainList {
		chainMap[chain.ChainID] = chain
	}

	// Step 7: Group RPC by chainID
	rpcMap := make(map[int64]rpc.RPCs)
	for _, chain := range chainList {
		urls := chain.RPC
		rpcs := rpc.NewRPCs(chain.ChainID, urls)
		rpcMap[chain.ChainID] = rpcs
	}

	return chainList, chainMap, rpcMap, nil
}

// getChainList fetches chain info from chainid.network
func getChainList() (ChainList, error) {
	resp, err := http.Get("https://chainid.network/chains.json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to fetch chain list")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var chainList ChainList
	if err := json.Unmarshal(body, &chainList); err != nil {
		return nil, err
	}

	return chainList, nil
}

// getLlamaChainHeaders fetches chain info from llama.fi and returns it as a map for easy lookup
func getLlamaChainHeaders() (map[int64]*LlamaChainHeader, error) {
	resp, err := http.Get("https://api.llama.fi/v2/chains")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to fetch llama chain headers")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var llamaChains LlamaChainHeaders
	if err := json.Unmarshal(body, &llamaChains); err != nil {
		return nil, err
	}

	llamaMap := make(map[int64]*LlamaChainHeader)
	for _, chain := range llamaChains {
		if chain.ChainID == nil {
			continue
		}
		chainIDNum, exact := toExactInt64(fmt.Sprint(chain.ChainID))
		if !exact {
			continue
		}
		// Check if the chainID already exists in the map
		if _, exists := llamaMap[chainIDNum]; exists {
			continue
		}
		llamaMap[chainIDNum] = chain

	}
	return llamaMap, nil
}

// getDetailLlamaChainInfo fetches RPC info for all chains from chainlist.org
func getDetailLlamaChainInfo() (map[int64]*LlamaChain, error) {
	// Send get request to https://chainlist.org
	resp, err := http.Get("https://chainlist.org")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// Read the html response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// Extract the RPC metadata from the html response body, get the substring between start with `{"chains":` and end with `},"__N_SSG":true}` (exclusive), there's only one such substring in the html response body
	start := []byte(`{"chains":`)
	end := []byte(`},"__N_SSG":true}`)
	jsonBody := body[bytes.Index(body, start)+len(start) : bytes.Index(body, end)]
	// Unmarshal the json string into LlamaChains
	var llamaChains LlamaChains
	if err := json.Unmarshal(jsonBody, &llamaChains); err != nil {
		return nil, err
	}
	// Convert LlamaChains to a map for easy lookup
	llamaChainsMap := make(map[int64]*LlamaChain)
	for _, chain := range llamaChains {
		llamaChainsMap[chain.ChainID] = chain
	}
	return llamaChainsMap, nil
}

// mergeRpcInfo merges RPC URLs from LlamaChains into ChainInfo, removing duplicates
func mergeRpcInfo(chainInfo *ChainInfo, llamaChainDetail *LlamaChain, privateRPCs []string) {
	urlMap := make(map[string]bool)
	for _, url := range chainInfo.RPC {
		urlMap[url] = true
	}
	for _, rpc := range llamaChainDetail.RPC {
		if _, exists := urlMap[rpc.URL]; !exists {
			chainInfo.RPC = append(chainInfo.RPC, rpc.URL)
			urlMap[rpc.URL] = true
		}
	}
	for _, rpc := range privateRPCs {
		if _, exists := urlMap[rpc]; !exists {
			chainInfo.RPC = append(chainInfo.RPC, rpc)
			urlMap[rpc] = true
		}
	}
	chainInfo.LlamaChainDetail = llamaChainDetail
}

func toExactInt64(n string) (int64, bool) {
	f, _, err := big.ParseFloat(n, 10, 0, big.ToZero)
	if err != nil {
		return 0, false
	}
	if !f.IsInt() {
		return 0, false
	}
	i, accuracy := f.Int64()
	if accuracy != big.Exact {
		return 0, false
	}
	return i, true
}
