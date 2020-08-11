package test

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/ipfs/test-plans/bitswap-tuning/utils"
)

// NOTE: To run use:
// ./testground run data-exchange/transfer --builder=docker:go --runner="local:docker" --dep="github.com/ipfs/go-bitswap=master"
// testground run single --plan=bitswap-tuning --testcase=baseline --builder=docker:go --runner=local:docker --instances=2

// Transfer data from S seeds to L leeches
func Transfer(runenv *runtime.RunEnv) error {
	// Test Parameters
	timeout := time.Duration(runenv.IntParam("timeout_secs")) * time.Second
	runTimeout := time.Duration(runenv.IntParam("run_timeout_secs")) * time.Second
	leechCount := runenv.IntParam("leech_count")
	passiveCount := runenv.IntParam("passive_count")
	requestStagger := time.Duration(runenv.IntParam("request_stagger")) * time.Millisecond
	bstoreDelay := time.Duration(runenv.IntParam("bstore_delay_ms")) * time.Millisecond
	runCount := runenv.IntParam("run_count")
	fileSizes, err := utils.ParseIntArray(runenv.StringParam("file_size"))
	if err != nil {
		return err
	}

	/// --- Set up
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := sync.MustBoundClient(ctx, runenv)
	nwClient := network.NewClient(client, runenv)

	/// --- Tear down
	defer func() {
		_, err := client.SignalAndWait(ctx, "end", runenv.TestInstanceCount)
		if err != nil {
			runenv.RecordFailure(err)
		} else {
			runenv.RecordSuccess()
		}
		client.Close()
	}()

	// Create libp2p node
	h, err := libp2p.New(ctx)
	if err != nil {
		return err
	}
	defer h.Close()
	runenv.RecordMessage("I am %s with addrs: %v", h.ID(), h.Addrs())

	peers := sync.NewTopic("peers", &peer.AddrInfo{})

	// Get sequence number of this host
	seq, err := client.Publish(ctx, peers, host.InfoFromHost(h))
	if err != nil {
		return err
	}
	// Type of node and identifiers assigned.
	grpseq, nodetp, tpindex, err := parseType(ctx, runenv, client, h, seq)
	if err != nil {
		return err
	}

	var seedIndex int64
	if nodetp == utils.Seed {
		if runenv.TestGroupID == "" {
			// If we're not running in group mode, calculate the seed index as
			// the sequence number minus the other types of node (leech / passive).
			// Note: sequence number starts from 1 (not 0)
			seedIndex = seq - int64(leechCount+passiveCount) - 1
		} else {
			// If we are in group mode, signal other seed nodes to work out the
			// seed index
			seedSeq, err := getNodeSetSeq(ctx, client, h, "seeds")
			if err != nil {
				return err
			}
			// Sequence number starts from 1 (not 0)
			seedIndex = seedSeq - 1
		}
	}

	// Get addresses of all peers
	peerCh := make(chan *peer.AddrInfo)
	sctx, cancelSub := context.WithCancel(ctx)
	if _, err := client.Subscribe(sctx, peers, peerCh); err != nil {
		cancelSub()
		return err
	}
	addrInfos, err := utils.AddrInfosFromChan(peerCh, runenv.TestInstanceCount)
	if err != nil {
		cancelSub()
		return fmt.Errorf("no addrs in %d seconds", timeout/time.Second)
	}
	cancelSub()

	/// --- Warm up

	// Set up network (with traffic shaping)
	latency, bandwidthMB, err := utils.SetupNetwork(ctx, runenv, nwClient, nodetp, tpindex)
	if err != nil {
		return fmt.Errorf("Failed to set up network: %w", err)
	}

	// Use the same blockstore on all runs for the seed node
	var bstore blockstore.Blockstore
	if nodetp == utils.Seed {
		bstore, err = utils.CreateBlockstore(ctx, bstoreDelay)
		if err != nil {
			return err
		}
	}

	// Signal that this node is in the given state, and wait for all peers to
	// send the same signal
	signalAndWaitForAll := func(state string) error {
		_, err := client.SignalAndWait(ctx, sync.State(state), runenv.TestInstanceCount)
		return err
	}

	// For each file size
	for sizeIndex, fileSize := range fileSizes {
		// If the total amount of seed data to be generated is greater than
		// parallelGenMax, generate seed data in series
		// genSeedSerial := seedCount > 2 || fileSize*seedCount > parallelGenMax
		genSeedSerial := true

		// Run the test runCount times
		var rootCid cid.Cid
		for runNum := 1; runNum < runCount+1; runNum++ {
			// Reset the timeout for each run
			ctx, cancel := context.WithTimeout(ctx, runTimeout)
			defer cancel()

			isFirstRun := runNum == 1
			runID := fmt.Sprintf("%d-%d", sizeIndex, runNum)

			// Wait for all nodes to be ready to start the run
			err = signalAndWaitForAll("start-run-" + runID)
			if err != nil {
				return err
			}

			runenv.RecordMessage("Starting run %d / %d (%d bytes)", runNum, runCount, fileSize)
			var bsnode *utils.Node
			// Create identifier for specific file size.
			rootCidTopic := getRootCidTopic(sizeIndex)

			switch nodetp {
			case utils.Seed:
				// For seeds, create a new bitswap node from the existing datastore
				bsnode, err = utils.CreateBitswapNode(ctx, h, bstore)
				if err != nil {
					return err
				}

				// If this is the first run for this file size
				if isFirstRun {
					seedGenerated := sync.State("seed-generated-" + runID)
					var start time.Time
					if genSeedSerial {
						// Each seed generates the seed data in series, to avoid
						// overloading a single machine hosting multiple instances
						if seedIndex > 0 {
							// Wait for the seeds with an index lower than this one
							// to generate their seed data
							doneCh := client.MustBarrier(ctx, seedGenerated, int(seedIndex)).C
							if err = <-doneCh; err != nil {
								return err
							}
						}

						// Generate a file of the given size and add it to the datastore
						start = time.Now()
					}
					runenv.RecordMessage("Generating seed data of %d bytes", fileSize)

					rootCid, err := setupSeed(ctx, runenv, bsnode, fileSize, int(seedIndex))
					if err != nil {
						return fmt.Errorf("Failed to set up seed: %w", err)
					}

					if genSeedSerial {
						runenv.RecordMessage("Done generating seed data of %d bytes (%s)", fileSize, time.Since(start))

						// Signal we've completed generating the seed data
						_, err = client.SignalEntry(ctx, seedGenerated)
						if err != nil {
							return fmt.Errorf("Failed to signal seed generated: %w", err)
						}
					}

					// Inform other nodes of the root CID
					if _, err = client.Publish(ctx, rootCidTopic, &rootCid); err != nil {
						return fmt.Errorf("Failed to get Redis Sync rootCidTopic %w", err)
					}
				}
			case utils.Leech:
				// For leeches, create a new blockstore on each run
				bstore, err = utils.CreateBlockstore(ctx, bstoreDelay)
				if err != nil {
					return err
				}

				// Create a new bitswap node from the blockstore
				bsnode, err = utils.CreateBitswapNode(ctx, h, bstore)
				if err != nil {
					return err
				}

				// If this is the first run for this file size
				if isFirstRun {
					// Get the root CID from a seed
					rootCidCh := make(chan *cid.Cid, 1)
					sctx, cancelRootCidSub := context.WithCancel(ctx)
					if _, err := client.Subscribe(sctx, rootCidTopic, rootCidCh); err != nil {
						cancelRootCidSub()
						return fmt.Errorf("Failed to subscribe to rootCidTopic %w", err)
					}

					// Note: only need to get the root CID from one seed - it should be the
					// same on all seeds (seed data is generated from repeatable random
					// sequence)
					rootCidPtr, ok := <-rootCidCh
					cancelRootCidSub()
					if !ok {
						return fmt.Errorf("no root cid in %d seconds", timeout/time.Second)
					}
					rootCid = *rootCidPtr
				}
			}

			// Wait for all nodes to be ready to dial
			err = signalAndWaitForAll("ready-to-connect-" + runID)
			if err != nil {
				return err
			}

			// Dial all peers
			dialed, err := utils.DialOtherPeers(ctx, h, addrInfos)
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

			var timeToFetch time.Duration
			if nodetp == utils.Leech {
				// Stagger the start of the first request from each leech
				// Note: seq starts from 1 (not 0)
				startDelay := time.Duration(seq-1) * requestStagger
				time.Sleep(startDelay)

				runenv.RecordMessage("Leech fetching data after %s delay", startDelay)
				start := time.Now()
				err := bsnode.FetchGraph(ctx, rootCid)
				timeToFetch = time.Since(start)
				if err != nil {
					return fmt.Errorf("Error fetching data through Bitswap: %w", err)
				}
				runenv.RecordMessage("Leech fetch complete (%s)", timeToFetch)
			}

			// Wait for all leeches to have downloaded the data from seeds
			err = signalAndWaitForAll("transfer-complete-" + runID)
			if err != nil {
				return err
			}

			/// --- Report stats
			err = emitMetrics(runenv, bsnode, runNum, seq, grpseq, latency, bandwidthMB, fileSize, nodetp, tpindex, timeToFetch)
			if err != nil {
				return err
			}

			// Shut down bitswap
			err = bsnode.Close()
			if err != nil {
				return fmt.Errorf("Error closing Bitswap: %w", err)
			}

			// Disconnect peers
			for _, c := range h.Network().Conns() {
				err := c.Close()
				if err != nil {
					return fmt.Errorf("Error disconnecting: %w", err)
				}
			}

			if nodetp == utils.Leech {
				// Free up memory by clearing the leech blockstore at the end of each run.
				// Note that although we create a new blockstore for the leech at the
				// start of the run, explicitly cleaning up the blockstore from the
				// previous run allows it to be GCed.
				if err := utils.ClearBlockstore(ctx, bstore); err != nil {
					return fmt.Errorf("Error clearing blockstore: %w", err)
				}
			}
		}
		if nodetp == utils.Seed {
			// Free up memory by clearing the seed blockstore at the end of each
			// set of tests over the current file size.
			if err := utils.ClearBlockstore(ctx, bstore); err != nil {
				return fmt.Errorf("Error clearing blockstore: %w", err)
			}
		}
	}

	/// --- Ending the test

	return nil
}
