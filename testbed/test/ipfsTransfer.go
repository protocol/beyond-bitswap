package test

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/adlrocha/beyond-bitswap/testbed/utils"
	"github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/interface-go-ipfs-core/path"
)

// NOTE: To run use:
// testground run single --plan=beyond-bitswap --testcase=ipfs-transfer --runner="local:exec" --builder=exec:go --instances=2

// IPFSTransfer data from S seeds to L leeches
func IPFSTransfer(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	// Test Parameters
	testvars := getEnvVars(runenv)

	/// --- Set up
	ctx, cancel := context.WithTimeout(context.Background(), testvars.Timeout)
	defer cancel()

	client := sync.MustBoundClient(ctx, runenv)
	nwClient := network.NewClient(client, runenv)

	/// --- Tear down
	// defer func() {
	// 	_, err := client.SignalAndWait(ctx, "end", runenv.TestInstanceCount)
	// 	if err != nil {
	// 		runenv.RecordFailure(err)
	// 	} else {
	// 		runenv.RecordSuccess()
	// 	}
	// 	client.Close()
	// }()

	runenv.RecordMessage("Preparing exchange for node: %v", testvars.ExchangeInterface)
	// Set exchange Interface
	exch, err := utils.SetExchange(ctx, testvars.ExchangeInterface)
	if err != nil {
		return err
	}
	nConfig, err := utils.GenerateAddrInfo(nwClient.MustGetDataNetworkIP().String())
	if err != nil {
		runenv.RecordMessage("Error generating node config")
		return err
	}
	// Create IPFS node
	ipfsNode, err := utils.CreateIPFSNodeWithConfig(ctx, nConfig, exch, testvars.DHTEnabled)
	// ipfsNode, err := utils.CreateIPFSNode(ctx)
	if err != nil {
		runenv.RecordFailure(err)
		return err
	}

	peers := sync.NewTopic("peers", &peer.AddrInfo{})

	// Get sequence number of this host
	seq, err := client.Publish(ctx, peers, *nConfig.AddrInfo)
	if err != nil {
		return err
	}
	// Type of node and identifiers assigned.
	grpseq, nodetp, tpindex, err := parseType(ctx, runenv, client, nConfig.AddrInfo, seq)
	if err != nil {
		return err
	}

	var seedIndex int64
	if nodetp == utils.Seed {
		if runenv.TestGroupID == "" {
			// If we're not running in group mode, calculate the seed index as
			// the sequence number minus the other types of node (leech / passive).
			// Note: sequence number starts from 1 (not 0)
			seedIndex = seq - int64(testvars.LeechCount+testvars.PassiveCount) - 1
		} else {
			// If we are in group mode, signal other seed nodes to work out the
			// seed index
			seedSeq, err := getNodeSetSeq(ctx, client, nConfig.AddrInfo, "seeds")
			if err != nil {
				return err
			}
			// Sequence number starts from 1 (not 0)
			seedIndex = seedSeq - 1
		}
	}
	runenv.RecordMessage("Seed index %v for: %v", &nConfig.AddrInfo.ID, seedIndex)

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
		return fmt.Errorf("no addrs in %d seconds", testvars.Timeout/time.Second)
	}
	cancelSub()
	runenv.RecordMessage("Got all addresses from other peers and network setup")

	/// --- Warm up

	// Set up network (with traffic shaping)
	latency, bandwidthMB, err := utils.SetupNetwork(ctx, runenv, nwClient, nodetp, tpindex)
	if err != nil {
		return fmt.Errorf("Failed to set up network: %v", err)
	}

	// Signal that this node is in the given state, and wait for all peers to
	// send the same signal
	signalAndWaitForAll := func(state string) error {
		_, err := client.SignalAndWait(ctx, sync.State(state), runenv.TestInstanceCount)
		return err
	}

	// According to the input data get the file size or the files to add.
	testFiles, err := utils.GetFileList(runenv)
	if err != nil {
		return err
	}
	runenv.RecordMessage("Got file list: %v", testFiles)

	err = signalAndWaitForAll("file-list-ready")
	if err != nil {
		return err
	}

	var runNum int
	var tcpFetch int64

	// For each file found in the test
	for fIndex, f := range testFiles {

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
			// TCP variables
			tcpAddrTopic := getTCPAddrTopic(fIndex)
			var tcpServer *utils.TCPServer

			switch nodetp {
			case utils.Seed:
				// Number of seeders to add the file
				rate := float64(testvars.SeederRate) / 100
				seeders := runenv.TestInstanceCount - (testvars.LeechCount + testvars.PassiveCount)
				toSeed := int(math.Ceil(float64(seeders) * rate))

				// If this is the first run for this file size.
				// Only a rate of seeders add the file.
				if isFirstRun && tpindex <= toSeed {
					// Generating and adding file to IPFS
					cid, err := generateAndAdd(ctx, runenv, ipfsNode, f)
					if err != nil {
						return err
					}
					runenv.RecordMessage("Published Added CID: %v", *cid)
					// Inform other nodes of the root CID
					if _, err = client.Publish(ctx, rootCidTopic, cid); err != nil {
						return fmt.Errorf("Failed to get Redis Sync rootCidTopic %w", err)
					}

					if testvars.TCPEnabled {
						runenv.RecordMessage("Starting TCP server in seed")
						// Start TCP server for file
						tcpServer, err = utils.SpawnTCPServer(ctx, nwClient.MustGetDataNetworkIP().String(), f)
						if err != nil {
							return fmt.Errorf("Failed to start tcpServer in seed %w", err)
						}
						// Inform other nodes of the TCPServerAddr
						runenv.RecordMessage("Publishing TCP address %v", tcpServer.Addr)
						if _, err = client.Publish(ctx, tcpAddrTopic, tcpServer.Addr); err != nil {
							return fmt.Errorf("Failed to get Redis Sync tcpAddr %w", err)
						}
						runenv.RecordMessage("Waiting to end finish TCP")
					}

				}
			case utils.Leech:
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
					// sequence or existing file)
					rootCidPtr, ok := <-rootCidCh
					if !ok {
						cancelRootCidSub()
						return fmt.Errorf("no root cid in %d seconds", testvars.Timeout/time.Second)
					}

					// Get TCP address from a seed
					// Only one leecher does it in order not to load the TCP server.
					if testvars.TCPEnabled && tpindex == 0 {
						tcpAddrCh := make(chan *string, 1)
						if _, err := client.Subscribe(sctx, tcpAddrTopic, tcpAddrCh); err != nil {
							cancelRootCidSub()
							return fmt.Errorf("Failed to subscribe to tcpServerTopic %w", err)
						}
						tcpAddrPtr, ok := <-tcpAddrCh

						runenv.RecordMessage("Received tcp server %v", tcpAddrPtr)
						if !ok {
							cancelRootCidSub()
							return fmt.Errorf("no tcp server addr received in %d seconds", testvars.Timeout/time.Second)
						}
						runenv.RecordMessage("Start fetching a TCP file from seed")
						start := time.Now()
						utils.FetchFileTCP(*tcpAddrPtr)
						tcpFetch = time.Since(start).Nanoseconds()
						runenv.RecordMessage("Fetched TCP file after %d (ns)", tcpFetch)
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

			// At this point TCP interactions are finished.
			if isFirstRun && nodetp == utils.Seed && testvars.TCPEnabled {
				runenv.RecordMessage("Closing TCP server")
				tcpServer.Close()
			}

			// dialed, err := ipfsNode.ConnectToPeers(ctx, runenv, addrInfos, maxConnections)
			dialed, err := utils.DialOtherPeers(ctx, ipfsNode.Node.PeerHost, addrInfos, testvars.MaxConnectionRate)
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
			if nodetp == utils.Leech {
				// Stagger the start of the first request from each leech
				// Note: seq starts from 1 (not 0)
				startDelay := time.Duration(seq-1) * testvars.RequestStagger

				runenv.RecordMessage("Starting to leech %d / %d (%d bytes)", runNum, testvars.RunCount, f.Size())
				runenv.RecordMessage("Leech fetching data after %s delay", startDelay)
				start := time.Now()
				// TODO: Here we may be able to define requesting pattern. ipfs.DAG()
				// Right now using a path.
				fPath := path.IpfsPath(rootCid)
				runenv.RecordMessage("Got path for file: %v", fPath)
				ctxFetch, cancel := context.WithTimeout(ctx, (testvars.RunTimeout/2)*time.Second)
				// Pin Add also traverse the whole DAG
				// err := ipfsNode.API.Pin().Add(ctxFetch, fPath)
				rcvFile, err := ipfsNode.API.Unixfs().Get(ctxFetch, fPath)
				if err != nil {
					runenv.RecordMessage("Error fetching data from IPFS: %w", err)
					leechFails++
				} else {
					err = files.WriteTo(rcvFile, "/tmp/"+strconv.Itoa(tpindex)+time.Now().String())
					if err != nil {
						cancel()
						return err
					}
					timeToFetch = time.Since(start).Nanoseconds()
					s, _ := rcvFile.Size()
					runenv.RecordMessage("Leech fetch of %d complete (%d ns)", s, timeToFetch)

				}
				// _, err := ipfsNode.API.Dag().Get(ctx, rootCid)
				cancel()
			}

			// Wait for all leeches to have downloaded the data from seeds
			err = signalAndWaitForAll("transfer-complete-" + runID)
			if err != nil {
				return err
			}

			/// --- Report stats
			err = ipfsNode.EmitMetrics(runenv, runNum, seq, grpseq, latency, bandwidthMB, int(f.Size()), nodetp, tpindex, timeToFetch, tcpFetch, leechFails, testvars.MaxConnectionRate)
			if err != nil {
				return err
			}
			runenv.RecordMessage("Finishing emitting metrics. Starting to clean...")

			// Disconnect peers
			for _, c := range ipfsNode.Node.PeerHost.Network().Conns() {
				err := c.Close()
				if err != nil {
					return fmt.Errorf("Error disconnecting: %w", err)
				}
			}
			runenv.RecordMessage("Closed Connections")

			if nodetp == utils.Leech {
				// Clearing datastore
				if err := ipfsNode.ClearDatastore(ctx, false); err != nil {
					return fmt.Errorf("Error clearing datastore: %w", err)
				}
			}
		}
		if nodetp == utils.Seed {
			// Between every file close the seed Node.
			// ipfsNode.Close()
			// runenv.RecordMessage("Closed Seed Node")
			if err := ipfsNode.ClearDatastore(ctx, false); err != nil {
				return fmt.Errorf("Error clearing datastore: %w", err)
			}

		}
	}

	runenv.RecordMessage("Ending testcase")
	return nil
}
