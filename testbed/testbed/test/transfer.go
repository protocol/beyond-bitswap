package test

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"

	"github.com/protocol/beyond-bitswap/testbed/testbed/utils"
)

// Transfer data from S seeds to L leeches
func Transfer(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	// Test Parameters
	testvars, err := getEnvVars(runenv)
	if err != nil {
		return err
	}
	bstoreDelay := time.Duration(runenv.IntParam("bstore_delay_ms")) * time.Millisecond

	/// --- Set up
	ctx, cancel := context.WithTimeout(context.Background(), testvars.Timeout)
	defer cancel()
	baseT, err := InitializeTest(ctx, runenv, testvars)
	if err != nil {
		return err
	}
	// Create libp2p node
	privKey, err := crypto.UnmarshalPrivateKey(baseT.nConfig.PrivKey)
	if err != nil {
		return err
	}

	h, err := libp2p.New(ctx, libp2p.Identity(privKey), libp2p.ListenAddrs(baseT.nConfig.AddrInfo.Addrs...))
	if err != nil {
		return err
	}
	defer h.Close()
	runenv.RecordMessage("I am %s with addrs: %v", h.ID(), h.Addrs())

	// Use the same blockstore on all runs for the seed node
	bstore, err := utils.CreateBlockstore(ctx, bstoreDelay)
	if err != nil {
		return err
	}
	// Create a new bitswap node from the blockstore
	bsnode, err := utils.CreateBitswapNode(ctx, h, bstore)
	if err != nil {
		return err
	}

	t := &NodeTestData{baseT, bsnode}

	// Signal that this node is in the given state, and wait for all peers to
	// send the same signal
	signalAndWaitForAll := t.signalAndWaitForAll
	t.stillAlive(runenv, testvars)

	var tcpFetch int64

	// For each permutation of network parameters (file size, bandwidth and latency)
	for pIndex, testParams := range testvars.Permutations {
		// Set up network (with traffic shaping)
		if err := utils.SetupNetwork(ctx, runenv, t.nwClient, t.nodetp, t.tpindex, testParams.Latency,
			testParams.Bandwidth, testParams.JitterPct); err != nil {
			return fmt.Errorf("Failed to set up network: %v", err)
		}

		// Run the test runCount times
		var rootCid cid.Cid
		var leechFails int64

		// Wait for all nodes to be ready to start the run
		err = signalAndWaitForAll(fmt.Sprintf("start-file-%d", pIndex))
		if err != nil {
			return err
		}

		switch t.nodetp {
		case utils.Seed:
			err = t.addPublishFile(ctx, pIndex, testParams.File, runenv, testvars)
		case utils.Leech:
			rootCid, err = t.readFile(ctx, pIndex, runenv, testvars)
		}
		if err != nil {
			return err
		}

		runenv.RecordMessage("File injest complete...")
		// Wait for all nodes to be ready to dial
		err = signalAndWaitForAll(fmt.Sprintf("injest-complete-%d", pIndex))
		if err != nil {
			return err
		}

		if testvars.TCPEnabled {
			runenv.RecordMessage("Running TCP test...")
			switch t.nodetp {
			case utils.Seed:
				err = t.runTCPServer(ctx, pIndex, testParams.File, runenv, testvars)
			case utils.Leech:
				tcpFetch, err = t.runTCPFetch(ctx, pIndex, runenv, testvars)
			}
			if err != nil {
				return err
			}
		}

		runenv.RecordMessage("Starting Bitswap Fetch...")

		for runNum := 1; runNum < testvars.RunCount+1; runNum++ {
			// Reset the timeout for each run
			ctx, cancel := context.WithTimeout(ctx, testvars.RunTimeout)
			defer cancel()

			runID := fmt.Sprintf("%d-%d", pIndex, runNum)

			// Wait for all nodes to be ready to start the run
			err = signalAndWaitForAll("start-run-" + runID)
			if err != nil {
				return err
			}

			runenv.RecordMessage("Starting run %d / %d (%d bytes)", runNum, testvars.RunCount, testParams.File.Size())

			// Dial all peers
			dialed, err := t.dialFn(ctx, h, t.nodetp, t.peerInfos, testvars.MaxConnectionRate)
			if err != nil {
				return err
			}
			runenv.RecordMessage("%s Dialed %d other nodes:", t.nodetp.String(), len(dialed))

			// Wait for all nodes to be connected
			err = signalAndWaitForAll("connect-complete-" + runID)
			if err != nil {
				return err
			}

			/// --- Start test

			var timeToFetch time.Duration
			if t.nodetp == utils.Leech {
				// Stagger the start of the first request from each leech
				// Note: seq starts from 1 (not 0)
				startDelay := time.Duration(t.seq-1) * testvars.RequestStagger
				time.Sleep(startDelay)

				runenv.RecordMessage("Leech fetching data after %s delay", startDelay)
				start := time.Now()
				err := bsnode.FetchGraph(ctx, rootCid)
				timeToFetch = time.Since(start)
				if err != nil {
					leechFails++
					runenv.RecordMessage("Error fetching data through Bitswap: %w", err)
				}
				runenv.RecordMessage("Leech fetch complete (%s)", timeToFetch)
			}

			// Wait for all leeches to have downloaded the data from seeds
			err = signalAndWaitForAll("transfer-complete-" + runID)
			if err != nil {
				return err
			}

			err = t.emitMetrics(runenv, runNum, testParams, timeToFetch, tcpFetch, leechFails, testvars.MaxConnectionRate)
			if err != nil {
				return err
			}
			runenv.RecordMessage("Finishing emitting metrics. Starting to clean...")

			// Disconnect peers
			err = t.cleanupRun(ctx, runenv)
			if err != nil {
				return err
			}
		}
		err = t.cleanupFile(ctx)
		if err != nil {
			return err
		}
	}
	runenv.RecordMessage("Ending testcase")

	/// --- Ending the test

	return nil
}
