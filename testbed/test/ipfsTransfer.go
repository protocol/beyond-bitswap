package test

import (
	"context"
	"fmt"
	"time"

	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/adlrocha/beyond-bitswap/testbed/utils"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/interface-go-ipfs-core/path"
)

// NOTE: To run use:
// testground run single --plan=beyond-bitswap --testcase=ipfs-transfer --runner="local:exec" --builder=exec:go --instances=2

// IPFSTransfer data from S seeds to L leeches
func IPFSTransfer(runenv *runtime.RunEnv) error {
	// Test Parameters
	exchangeInterface := runenv.StringParam("exchange_interface")
	timeout := time.Duration(runenv.IntParam("timeout_secs")) * time.Second
	runTimeout := time.Duration(runenv.IntParam("run_timeout_secs")) * time.Second
	leechCount := runenv.IntParam("leech_count")
	passiveCount := runenv.IntParam("passive_count")
	requestStagger := time.Duration(runenv.IntParam("request_stagger")) * time.Millisecond
	// bstoreDelay := time.Duration(runenv.IntParam("bstore_delay_ms")) * time.Millisecond
	runCount := runenv.IntParam("run_count")
	maxConnectionRate := runenv.IntParam("max_connection_rate")
	// coreNode := runenv.StringParam("core_node")

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

	runenv.RecordMessage("Preparing exchange for node: %v", exchangeInterface)
	// // Set exchange Interface
	// exch, err := utils.SetExchange(ctx, exchangeInterface)
	// if err != nil {
	// 	return err
	// }
	// // Create IPFS node
	// ipfsNode, err := utils.NewNode(ctx, exch)
	ipfsNode, err := utils.CreateIPFSNode(ctx)
	if err != nil {
		runenv.RecordFailure(err)
		return err
	}
	defer ipfsNode.Node.Close()
	h := ipfsNode.Node.PeerHost
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
	runenv.RecordMessage("Seed index %v for: %v", h.ID(), seedIndex)

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
		return fmt.Errorf("Failed to set up network: %v", err)
	}

	// Signal that this node is in the given state, and wait for all peers to
	// send the same signal
	signalAndWaitForAll := func(state string) error {
		_, err := client.SignalAndWait(ctx, sync.State(state), runenv.TestInstanceCount)
		return err
	}
	runenv.RecordMessage("Got all addresses from other peers and network setup")

	// According to the input data get the file size or the files to add.
	files, err := utils.GetFileList(runenv)
	if err != nil {
		return err
	}
	runenv.RecordMessage("Got file list: %v", files)

	var runNum int
	var fPath path.Resolved
	var tcpFetch int64

	// For each file found in the test
	for fIndex, f := range files {
		var rootCid cid.Cid
		// Run the test runcount times
		for runNum = 1; runNum < runCount+1; runNum++ {
			// Reset the timeout for each run
			ctx, cancel := context.WithTimeout(ctx, runTimeout)
			defer cancel()

			isFirstRun := runNum == 1
			runID := fmt.Sprintf("%d-%d", fIndex, runNum)

			// Wait for all nodes to be ready to start the run
			err = signalAndWaitForAll("start-run-" + runID)
			if err != nil {
				return err
			}

			runenv.RecordMessage("Starting run %d / %d (%d bytes)", runNum, runCount, f.Size)

			// Create identifier for specific file size.
			rootCidTopic := getRootCidTopic(fIndex)
			tcpAddrTopic := getTCPAddrTopic(fIndex)
			switch nodetp {
			case utils.Seed:
				// If this is the first run for this file size
				// TODO: For now all seeds in the network have the files.
				if isFirstRun {
					// Generate the file
					tmpFile, err := ipfsNode.GenerateFile(ctx, runenv, f)
					if err != nil {
						return err
					}
					// Add file to the IPFS network
					path, err := ipfsNode.Add(ctx, runenv, tmpFile)
					cid := path.Cid()
					if err != nil {
						return err
					}
					// Inform other nodes of the root CID
					if _, err = client.Publish(ctx, rootCidTopic, &cid); err != nil {
						return fmt.Errorf("Failed to get Redis Sync rootCidTopic %w", err)
					}

					runenv.RecordMessage("Starting TCP server in seed")
					// Start TCP server for file
					tcpServer, err := utils.SpawnTCPServer(ctx, tmpFile)
					if err != nil {
						return fmt.Errorf("Failed to start tcpServer in seed %w", err)
					}
					// Inform other nodes of the TCPServerAddr
					if _, err = client.Publish(ctx, tcpAddrTopic, tcpServer.Addr); err != nil {
						return fmt.Errorf("Failed to get Redis Sync tcpAddr %w", err)
					}
					runenv.RecordMessage("Waiting to end finish TCP")
					// Wait for everyone to fetch TCP and close server.
					err = signalAndWaitForAll("finished-tcp" + runID)
					if err != nil {
						return err
					}
					tcpServer.Close()
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
						return fmt.Errorf("no root cid in %d seconds", timeout/time.Second)
					}

					// Get TCP address from a seed
					tcpAddrCh := make(chan string, 1)
					if _, err := client.Subscribe(sctx, tcpAddrTopic, tcpAddrCh); err != nil {
						cancelRootCidSub()
						return fmt.Errorf("Failed to subscribe to tcpServerTopic %w", err)
					}
					tcpAddrPtr, ok := <-tcpAddrCh
					if !ok {
						cancelRootCidSub()
						return fmt.Errorf("no tcp server addr received in %d seconds", timeout/time.Second)
					}
					cancelRootCidSub()
					runenv.RecordMessage("Received tcp server and rootCID")
					rootCid = *rootCidPtr
					runenv.RecordMessage("Received rootCid: %v", rootCid)
					runenv.RecordMessage("Start fetching a TCP file from seed")
					start := time.Now()
					utils.FetchFileTCP(tcpAddrPtr)
					tcpFetch = time.Since(start).Nanoseconds()
					runenv.RecordMessage("Fetched TCP file after %d (ns)", tcpFetch)

					// Signalled that finished fetching TCP.
					err = signalAndWaitForAll("finished-tcp" + runID)
					if err != nil {
						return err
					}
				}
			}

			// Wait for all nodes to be ready to dial
			err = signalAndWaitForAll("ready-to-connect-" + runID)
			if err != nil {
				return err
			}

			// Start peer connection. Connections are performed randomly in ConnectToPeers
			maxConnections := maxConnectionRate * runenv.TestInstanceCount
			// dialed, err := ipfsNode.ConnectToPeers(ctx, runenv, addrInfos, maxConnections)
			// TODO: MaxConnections not working
			dialed, err := utils.DialOtherPeers(ctx, h, addrInfos, maxConnections)
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

				runenv.RecordMessage("Starting to leech %d / %d (%d bytes)", runNum, runCount, f.Size)
				runenv.RecordMessage("Leech fetching data after %s delay", startDelay)
				start := time.Now()
				// TODO: Here we may be able to define requesting pattern. ipfs.DAG()
				// Right now using a path.
				fPath = path.IpfsPath(rootCid)
				runenv.RecordMessage("Got path for file: %v", fPath)
				rcvFile, err := ipfsNode.API.Unixfs().Get(ctx, fPath)
				// _, err := ipfsNode.API.Dag().Get(ctx, rootCid)
				timeToFetch = time.Since(start)
				if err != nil {
					runenv.RecordMessage("Error fetching data through IPFS: %w", err)
					return fmt.Errorf("Error fetching data through IPFS: %w", err)
				}
				s, _ := rcvFile.Size()
				runenv.RecordMessage("Leech fetch of %d complete (%s)", s, timeToFetch)
			}

			// Wait for all leeches to have downloaded the data from seeds
			err = signalAndWaitForAll("transfer-complete-" + runID)
			if err != nil {
				return err
			}

			/// --- Report stats
			err = ipfsNode.EmitMetrics(runenv, runNum, seq, grpseq, latency, bandwidthMB, int(f.Size), nodetp, tpindex, timeToFetch, tcpFetch)
			if err != nil {
				return err
			}
			runenv.RecordMessage("Finishing emitting metrics. Starting to clean...")

			// Disconnect peers
			for _, c := range h.Network().Conns() {
				err := c.Close()
				if err != nil {
					return fmt.Errorf("Error disconnecting: %w", err)
				}
			}
			runenv.RecordMessage("Closed Connections")
			err = signalAndWaitForAll("metrics-complete-" + runID)

			if nodetp == utils.Leech {
				// Free up memory by clearing the leech blockstore at the end of each run.
				// Note that although we create a new blockstore for the leech at the
				// start of the run, explicitly cleaning up the blockstore from the
				// previous run allows it to be GCed.
				runenv.RecordMessage("Cleaning Leech Blockstore and Datastore")
				// if err := utils.ClearBlockstore(ctx, ipfsNode.Node.Blockstore); err != nil {
				// 	return fmt.Errorf("Error clearing blockstore: %w", err)
				// }

				if err := ipfsNode.ClearDatastore(ctx, false); err != nil {
					return fmt.Errorf("Error clearing datastore: %w", err)
				}
			}
		}
		if nodetp == utils.Seed {
			// Free up memory by clearing the seed blockstore at the end of each
			// set of tests over the current file size.
			runenv.RecordMessage("Cleaning Seed Blockstore")
			// if err := utils.ClearBlockstore(ctx, ipfsNode.Node.Blockstore); err != nil {
			// 	return fmt.Errorf("Error clearing blockstore: %w", err)
			// }
			if err := ipfsNode.ClearDatastore(ctx, false); err != nil {
				return fmt.Errorf("Error clearing datstore: %w", err)
			}
		}
	}

	runenv.RecordMessage("Ending testcase")
	return nil
}
