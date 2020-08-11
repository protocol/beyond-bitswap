package test

import "github.com/testground/sdk-go/runtime"

func Test(runenv *runtime.RunEnv) error {
	i := 0
	for i < 10 {
		runenv.RecordMessage("I am testing: ", runenv.TestCase)
		runenv.R().RecordPoint("pointer", float64(2213231*i+i))
		i++
	}

	return nil
}
