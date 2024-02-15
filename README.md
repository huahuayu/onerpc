# 1rpc 

There are many free evm rpcs on the web (you can find them [here](https://chainlist.org)).

1rpc integrate them all, which provide higher availability, better performance, and better privacy.

Currently, on 15,Feb,2024, there are 1276 chains with 2693 free rpcs are integrated, and the number is growing.

https://1rpc.dev is the public gateway, you can use it directly.

### What's the problem with free rpcs

The problems are:
- Rate limits. If you just use one of them, you may hit rate limit quickly. 
- Single point of failure. The free rpcs are not reliable, they may be down, or return error, etc.
- Complexity. Yes, you use multiple of them, but you need to handle load balancing, multi client switching, error handling, etc... brings a lot more complexity to your code.
- Privacy. It's easy to link your identity to the specific rpc.

### The solution

1rpc provide a gateway to integrate multiple free rpcs, balancing the load, increasing the availability and create a rpc pool for better privacy.

You can use the gateway as a drop-in replacement for the original rpc, without code change.

The endpoint is `http://gateway-host:port/chain/:chainId`.

For example: ethereum is `http://gateway-host:port/chain/1`, and bsc is `http://gateway-host:port/chain/56`, etc.

## Work flows

1. When you send a request to the gateway, it will pick a random free rpc to send the request to.
2. The static rpc response will be cached for better performance e.g. getTransactionByHash/getTransactionReceipt.
3. If the free rpc returns an error, the gateway will try another free rpc.
4. You can add your own rpcs additionally to the free ones.

## Getting started

Clone the project:

```shell
git clone https://github.com/huahuayu/onerpc.git
```

### Local build

```shell
cd onerpc
make build
cd bin
```

Run the binary:

```shell
rpc_gateway --port=8080
```

### Run in docker

Set the environment variables:

```shell
cp .env.example .env # and modify the .env file
```

Deploy by docker-compose:

```shell
cd onerpc
docker-compose up -d --build
```

Use the gateway:

```shell
curl --location 'http://localhost:8080/chain/1' \
--header 'Content-Type: application/json' \
--data '{
	"jsonrpc":"2.0",
	"method":"eth_getTransactionByHash",
	"params":[
		"0x9d0cf5efc84d3fbd075fe93cf15b01982c332242adbf831c51a15f25eb1160fa"
	],
	"id":1
}'
```

## Add your own rpc

You can add your own rpcs additionally to the free rpcs, so the gateway will use them as well.

```shell
rpc_gateway --rpcs='[{"chainID":1,"rpc":["https://inhouse_rpc1.com","https://inhouse_rpc2.io"]}]'
```

## Metrics

The gateway provides prometheus metrics, enable it by `--metrics`.

```shell
rpc_gateway --port=8080 --metrics --metricsPort=9090
```

## Robust test

Test the gateway with 100 block's transactions & receipts fetching, about 30,000 requests in total.

```
eth/eth_test.go
```

The workload is distributed to multiple rpcs effectively, the gateway handle the requests quite smoothly.

Especially if you configured fallback rpcs (check the `--fallback` flag), the gateway can be even more robust.

## License

[MIT](https://github.com/huahuayu/onerpc/blob/main/license.txt)