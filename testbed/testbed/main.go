package main

import (
	test "github.com/protocol/beyond-bitswap/testbed/testbed/test"
	"github.com/testground/sdk-go/run"
)

func main() {
	run.InvokeMap(map[string]interface{}{
		"transfer":     test.Transfer,
		"tcp-transfer": test.TCPTransfer,
	})
}
