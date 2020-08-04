package main

import (
	test "github.com/ipfs/test-plans/bitswap-tuning/test"
	"github.com/testground/sdk-go/run"
)

func main() {
	run.InvokeMap(map[string]interface{}{
		"transfer": test.Transfer,
		"fuzz":     test.Fuzz,
		"baseline": test.Baseline,
	})
}

// func run(runenv *runtime.RunEnv) error {
// 	switch c := runenv.TestCase; c {
// 	case "transfer":
// 		return test.Transfer(runenv)
// 	case "baseline":
// 		return test.Baseline(runenv)
// 	case "fuzz":
// 		return test.Fuzz(runenv)
// 	default:
// 		panic("unrecognized test case")
// 	}
// }
