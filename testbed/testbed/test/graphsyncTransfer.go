package test

import (
	"context"
	"fmt"
	"time"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"

	"github.com/ipfs/go-cid"
	"github.com/protocol/beyond-bitswap/testbed/testbed/utils"
)

// GraphsyncTransfer data from S seeds to L leeches
func GraphsyncTransfer(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	// Test Parameters
	testvars, err := getEnvVars(runenv)
	if err != nil {
		return err
	}

	/// --- Set up
	ctx, cancel := context.WithTimeout(context.Background(), testvars.Timeout)
	defer cancel()
	t, err := InitializeIPFSTest(ctx, runenv, testvars)
	if err != nil {
		return err
	}
	ipfsNode := t.ipfsNode
	signalAndWaitForAll := t.signalAndWaitForAll

	// Start still alive process if enabled
	t.stillAlive(runenv, testvars)

	var runNum int
	var tcpFetch int64

	// For each file found in the test
	for pIndex, testParams := range testvars.Permutations {
		// Set up network (with traffic shaping)
		if err := utils.SetupNetwork(ctx, runenv, t.nwClient, t.nodetp, t.tpindex, testParams.Latency,
			testParams.Bandwidth, testParams.JitterPct); err != nil {
			return fmt.Errorf("Failed to set up network: %v", err)
		}

		// Accounts for every file that couldn't be found.
		var leechFails int64
		var rootCid cid.Cid

		// Wait for all nodes to be ready to start the file
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

		// Run the test runcount times
		for runNum = 1; runNum < testvars.RunCount+1; runNum++ {
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

			dialed, err := t.dialFn(ctx, ipfsNode.Node.PeerHost, t.nodetp, t.peerInfos, testvars.MaxConnectionRate)
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
			var timeToFetch int64
			if t.nodetp == utils.Leech {
				// Stagger the start of the first request from each leech
				// Note: seq starts from 1 (not 0)
				targetPeer := t.peerInfos[0].Addr
				if targetPeer.ID == t.nConfig.AddrInfo.ID {
					targetPeer = t.peerInfos[1].Addr
				}
				startDelay := time.Duration(t.seq-1) * testvars.RequestStagger
				runenv.RecordMessage("Starting to leech %d / %d (%d bytes)", runNum, testvars.RunCount, testParams.File.Size())
				runenv.RecordMessage("Leech fetching data after %s delay", startDelay)
				start := time.Now()
				// TODO: Here we may be able to define requesting pattern. ipfs.DAG()
				// Right now using a path.
				runenv.RecordMessage("Got path for file: %v", rootCid)
				// Get Graphsync
				err := ipfsNode.GetGraphsync(ctx, targetPeer, rootCid)
				timeToFetch = time.Since(start).Nanoseconds()
				runenv.RecordMessage("Leech fetch complete (%d ns)", timeToFetch)

				if err != nil {
					runenv.RecordMessage("Error fetching data from IPFS: %w", err)
					leechFails++
				} else {
					timeToFetch = time.Since(start).Nanoseconds()
					runenv.RecordMessage("Leech fetch complete (%d ns)", timeToFetch)
				}
				cancel()
			}

			// Wait for all leeches to have downloaded the data from seeds
			err = signalAndWaitForAll("transfer-complete-" + runID)
			if err != nil {
				return err
			}

			/// --- Report stats
			err = ipfsNode.EmitMetrics(runenv, runNum, t.seq, t.grpseq, testParams.Latency, testParams.Bandwidth,
				int(testParams.File.Size()), t.nodetp, t.tpindex, timeToFetch, tcpFetch, leechFails, testvars.MaxConnectionRate)
			if err != nil {
				return err
			}
			runenv.RecordMessage("Finishing emitting metrics. Starting to clean...")

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
	return nil
}
