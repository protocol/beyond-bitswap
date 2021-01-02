package main

import (
	test "github.com/protocol/beyond-bitswap/testbed/testbed/test"
	"github.com/testground/sdk-go/run"
)

func main() {
	run.InvokeMap(map[string]interface{}{
		"bitswap-transfer":   test.Transfer,
		"ipfs-transfer":      test.IPFSTransfer,
		"tcp-transfer":       test.TCPTransfer,
		"graphsync-transfer": test.GraphsyncTransfer,
	})
}
