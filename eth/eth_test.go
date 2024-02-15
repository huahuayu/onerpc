package eth

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"golang.org/x/time/rate"
	"math/big"
	"testing"
	"time"
)

func TestGatewayPerformance(t *testing.T) {
	client := NewClient("http://localhost:8080/chain/1")

	// Define the rate limit to 100 requests per second.
	rateLimit := rate.NewLimiter(rate.Every(10*time.Millisecond), 1) // 100 rps

	var total int
	for i := 19107800; i < 19107900; i++ {
		// Wait for permission to proceed or abort the test if not allowed to proceed.
		if err := rateLimit.Wait(context.Background()); err != nil {
			t.Fatal("Rate limiter error:", err)
		}

		block, err := client.Client.BlockByNumber(context.Background(), big.NewInt(int64(i)))
		if err != nil {
			t.Fatal("Error fetching block:", err)
		}
		total++
		for _, tx := range block.Transactions() {
			transactionChan := make(chan *types.Transaction)
			receiptChan := make(chan *types.Receipt)
			errChan := make(chan error)

			go func(txHash common.Hash) {
				// Enforce rate limiting for transaction fetch.
				if err := rateLimit.Wait(context.Background()); err != nil {
					errChan <- err
					return
				}

				transaction, _, err := client.Client.TransactionByHash(context.Background(), txHash)
				total++
				if err != nil {
					errChan <- err
				} else {
					transactionChan <- transaction
				}
			}(tx.Hash())

			go func(txHash common.Hash) {
				// Enforce rate limiting for receipt fetch.
				if err := rateLimit.Wait(context.Background()); err != nil {
					errChan <- err
					return
				}

				receipt, err := client.Client.TransactionReceipt(context.Background(), txHash)
				total++
				if err != nil {
					errChan <- err
				} else {
					receiptChan <- receipt
				}
			}(tx.Hash())

			select {
			case transaction := <-transactionChan:
				t.Log(block.Number(), transaction.Hash(), total)
			case receipt := <-receiptChan:
				t.Log(block.Number(), receipt.TransactionIndex, total)
			case err := <-errChan:
				t.Error(tx.Hash(), err)
				continue
			}
		}
	}
}
