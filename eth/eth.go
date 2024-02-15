package eth

import (
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

type Client struct {
	Client *ethclient.Client
}

func NewClient(url string) *Client {
	rpcClient, err := rpc.Dial(url)
	if err != nil {
		panic(err)
	}
	client := ethclient.NewClient(rpcClient)
	if err != nil {
		panic(err)
	}
	return &Client{
		Client: client,
	}
}
