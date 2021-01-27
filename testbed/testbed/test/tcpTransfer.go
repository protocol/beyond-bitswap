package test

import (
	"context"
	"fmt"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"

	"github.com/protocol/beyond-bitswap/testbed/testbed/utils"
)

// IPFSTransfer data from S seeds to L leeches
func TCPTransfer(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	// Test Parameters
	testvars, err := getEnvVars(runenv)
	if err != nil {
		return err
	}

	/// --- Set up
	ctx, cancel := context.WithTimeout(context.Background(), testvars.Timeout)
	defer cancel()
	t, err := InitializeTest(ctx, runenv, testvars)
	if err != nil {
		return err
	}

	// Signal that this node is in the given state, and wait for all peers to
	// send the same signal
	signalAndWaitForAll := t.signalAndWaitForAll

	err = signalAndWaitForAll("file-list-ready")
	if err != nil {
		return err
	}

	var tcpFetch int64

	// For each file found in the test
	for pIndex, testParams := range testvars.Permutations {
		// Set up network (with traffic shaping)
		if err := utils.SetupNetwork(ctx, runenv, t.nwClient, t.nodetp, t.tpindex, testParams.Latency,
			testParams.Bandwidth, testParams.JitterPct); err != nil {
			return fmt.Errorf("Failed to set up network: %v", err)
		}

		err = signalAndWaitForAll(fmt.Sprintf("transfer-start-%d", pIndex))
		if err != nil {
			return err
		}

		switch t.nodetp {
		case utils.Seed:
			err = t.runTCPServer(ctx, pIndex, testParams.File, runenv, testvars)
		case utils.Leech:
			tcpFetch, err = t.runTCPFetch(ctx, pIndex, runenv, testvars)
			runenv.R().RecordPoint(fmt.Sprintf("%s/name:time_to_fetch", t.nodetp), float64(tcpFetch))
		}
		if err != nil {
			return err
		}
	}

	runenv.RecordMessage("Ending testcase")
	return nil
}
