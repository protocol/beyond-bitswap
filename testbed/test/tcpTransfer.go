package test

import (
	"context"
	"fmt"
	"time"

	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"

	"github.com/adlrocha/beyond-bitswap/testbed/utils"
)

// IPFSTransfer data from S seeds to L leeches
func TCPTransfer(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	// Test Parameters
	jitterPct := runenv.IntParam("jitter_pct")
	bandwidth := runenv.IntParam("bandwidth_mb")
	latency := time.Duration(runenv.IntParam("latency_ms"))
	seq := initCtx.GlobalSeq

	/// --- Set up
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	initCtx.MustWaitAllInstancesInitialized(ctx)

	client := sync.MustBoundClient(ctx, runenv)
	nwClient := network.NewClient(client, runenv)

	cfg := &network.Config{
		Network:       "default",
		Enable:        true,
		RoutingPolicy: network.AllowAll,
		Default: network.LinkShape{
			Latency:   latency,
			Bandwidth: uint64(bandwidth) * 1024 * 1024,
			Jitter:    (time.Duration(jitterPct) * latency) / 100,
		},
		CallbackState:  sync.State("network-configured"),
		CallbackTarget: runenv.TestInstanceCount,
	}

	nwClient.ConfigureNetwork(ctx, cfg)

	var nodetp string
	if seq%2 == 0 {
		nodetp = "seed"
	} else {
		nodetp = "leech"
	}
	runenv.RecordMessage("I am node type: %s", nodetp)

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

	var tcpFetch int64

	// For each file found in the test
	for fIndex, f := range testFiles {

		// Wait for all nodes to be ready to start the run
		err = signalAndWaitForAll(fmt.Sprintf("transfer-complete-%d", fIndex))
		if err != nil {
			return err
		}

		var tcpServer *utils.TCPServer
		tcpAddrTopic := getTCPAddrTopic(fIndex)

		switch nodetp {
		case "seed":
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
		case "leech":
			tcpAddrCh := make(chan *string, 1)
			if _, err := client.Subscribe(ctx, tcpAddrTopic, tcpAddrCh); err != nil {
				return fmt.Errorf("Failed to subscribe to tcpServerTopic %w", err)
			}
			tcpAddrPtr, ok := <-tcpAddrCh

			runenv.RecordMessage("Received tcp server %v", tcpAddrPtr)
			if !ok {
				return fmt.Errorf("no tcp server addr received in %d seconds", 1000)
			}
			runenv.RecordMessage("Start fetching a TCP file from seed")
			start := time.Now()
			utils.FetchFileTCP(*tcpAddrPtr)
			tcpFetch = time.Since(start).Nanoseconds()
			runenv.RecordMessage("Fetched TCP file after %d (ns)", tcpFetch)
		}

		// Wait for all leeches to have downloaded the data from seeds
		err = signalAndWaitForAll(fmt.Sprintf("transfer-complete-%d", fIndex))
		if err != nil {
			return err
		}
		runenv.R().RecordPoint(fmt.Sprintf("%s/name:time_to_fetch", nodetp), float64(tcpFetch))

		// At this point TCP interactions are finished.
		if nodetp == "seed" {
			runenv.RecordMessage("Closing TCP server")
			tcpServer.Close()
		}
	}

	runenv.RecordMessage("Ending testcase")
	return nil
}
