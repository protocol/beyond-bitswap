package test

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"

	"github.com/adlrocha/beyond-bitswap/testbed/utils"
	"github.com/ipfs/go-cid"
)

// GraphsyncTransfer data from S seeds to L leeches
func GraphsyncTransfer(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	// Test Parameters
	testvars := getEnvVars(runenv)
	/// --- Set up
	ctx, cancel := context.WithTimeout(context.Background(), testvars.Timeout)
	defer cancel()
	t, err := InitializeTest(ctx, runenv, testvars)
	if err != nil {
		return err
	}
	ipfsNode := t.ipfsNode
	signalAndWaitForAll := t.signalAndWaitForAll

	// Start still alive process if enabled
	t.stillAlive(runenv, testvars)

	var runNum int

	// For each file found in the test
	for fIndex, f := range t.testFiles {

		// Accounts for every file that couldn't be found.
		var leechFails int64
		var rootCid cid.Cid
		// Run the test runcount times
		for runNum = 1; runNum < testvars.RunCount+1; runNum++ {
			// Reset the timeout for each run
			ctx, cancel := context.WithTimeout(ctx, testvars.RunTimeout)
			defer cancel()

			isFirstRun := runNum == 1
			runID := fmt.Sprintf("%d-%d", fIndex, runNum)

			// Wait for all nodes to be ready to start the run
			err = signalAndWaitForAll("start-run-" + runID)
			if err != nil {
				return err
			}

			runenv.RecordMessage("Starting run %d / %d (%d bytes)", runNum, testvars.RunCount, f.Size())
			// Create identifier for specific file size.
			rootCidTopic := getRootCidTopic(fIndex)

			switch t.nodetp {
			case utils.Seed:
				// Number of seeders to add the file
				rate := float64(testvars.SeederRate) / 100
				seeders := runenv.TestInstanceCount - (testvars.LeechCount + testvars.PassiveCount)
				toSeed := int(math.Ceil(float64(seeders) * rate))

				// If this is the first run for this file size.
				// Only a rate of seeders add the file.
				if isFirstRun && t.tpindex <= toSeed {
					// Generating and adding file to IPFS
					cid, err := generateAndAdd(ctx, runenv, ipfsNode, f)
					if err != nil {
						return err
					}
					runenv.RecordMessage("Published Added CID: %v", *cid)
					// Inform other nodes of the root CID
					if _, err = t.client.Publish(ctx, rootCidTopic, cid); err != nil {
						return fmt.Errorf("Failed to get Redis Sync rootCidTopic %w", err)
					}

				}
			case utils.Leech:
				// If this is the first run for this file size
				if isFirstRun {
					// Get the root CID from a seed
					rootCidCh := make(chan *cid.Cid, 1)
					sctx, cancelRootCidSub := context.WithCancel(ctx)
					if _, err := t.client.Subscribe(sctx, rootCidTopic, rootCidCh); err != nil {
						cancelRootCidSub()
						return fmt.Errorf("Failed to subscribe to rootCidTopic %w", err)
					}
					// Note: only need to get the root CID from one seed - it should be the
					// same on all seeds (seed data is generated from repeatable random
					// sequence or existing file)
					rootCidPtr, ok := <-rootCidCh
					if !ok {
						cancelRootCidSub()
						return fmt.Errorf("no root cid in %d seconds", testvars.Timeout/time.Second)
					}

					rootCid = *rootCidPtr
					runenv.RecordMessage("Received rootCid: %v", rootCid)
					cancelRootCidSub()
				}
			}

			runenv.RecordMessage("Ready to start connecting...")
			// Wait for all nodes to be ready to dial
			err = signalAndWaitForAll("ready-to-connect-" + runID)
			if err != nil {
				return err
			}

			// dialed, err := ipfsNode.ConnectToPeers(ctx, runenv, addrInfos, maxConnections)
			dialed, err := utils.DialOtherPeers(ctx, ipfsNode.Node.PeerHost, t.addrInfos, testvars.MaxConnectionRate)
			if err != nil {
				return err
			}
			runenv.RecordMessage("Dialed %d other nodes", len(dialed))

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
				targetPeer := t.addrInfos[0]
				if targetPeer.ID == t.nConfig.AddrInfo.ID {
					targetPeer = t.addrInfos[1]
				}
				startDelay := time.Duration(t.seq-1) * testvars.RequestStagger
				runenv.RecordMessage("Starting to leech %d / %d (%d bytes)", runNum, testvars.RunCount, f.Size())
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
		}
	}

	runenv.RecordMessage("Ending testcase")
	return nil
}
