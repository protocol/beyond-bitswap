package main

import (
	test "github.com/adlrocha/beyond-bitswap/testbed/test"
	"github.com/testground/sdk-go/run"
)

func main() {
	run.InvokeMap(map[string]interface{}{
		"bitswap-transfer":   test.Transfer,
		"ipfs-transfer":      test.IPFSTransfer,
		"tcp-transfer":       test.TCPTransfer,
		"waves":              test.Waves,
		"sparse":             test.Sparse,
		"graphsync-transfer": test.GraphsyncTransfer,
	})
}
