package rpc

import (
	"testing"
)

func TestRPC_UpdateHeight(t *testing.T) {
	rpc := NewRPC(1, "https://eth.llamarpc.com") // Replace with your actual RPC URL
	err := rpc.updateHeight()
	if err != nil {
		t.Error(err)
	}
	if rpc.Height == 0 {
		t.Error("Height not updated")
	}
	t.Log(rpc.Height)
}
