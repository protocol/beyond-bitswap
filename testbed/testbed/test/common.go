package test

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/protocol/beyond-bitswap/testbed/testbed/utils"
	"github.com/protocol/beyond-bitswap/testbed/testbed/utils/dialer"

	"github.com/testground/sdk-go/network"
)

type TestPermutation struct {
	File      utils.TestFile
	Bandwidth int
	Latency   time.Duration
	JitterPct int
}

// TestVars testing variables
type TestVars struct {
	ExchangeInterface string
	Timeout           time.Duration
	RunTimeout        time.Duration
	LeechCount        int
	PassiveCount      int
	RequestStagger    time.Duration
	RunCount          int
	MaxConnectionRate int
	TCPEnabled        bool
	SeederRate        int
	DHTEnabled        bool
	LlEnabled         bool
	Dialer            string
	NumWaves          int
	Permutations      []TestPermutation
}

type TestData struct {
	client              *sync.DefaultClient
	nwClient            *network.Client
	nConfig             *utils.NodeConfig
	peerInfos           []dialer.PeerInfo
	dialFn              dialer.Dialer
	signalAndWaitForAll func(state string) error
	seq                 int64
	grpseq              int64
	nodetp              utils.NodeType
	tpindex             int
	seedIndex           int64
}

func getEnvVars(runenv *runtime.RunEnv) (*TestVars, error) {
	tv := &TestVars{}
	if runenv.IsParamSet("exchange_interface") {
		tv.ExchangeInterface = runenv.StringParam("exchange_interface")
	}
	if runenv.IsParamSet("timeout_secs") {
		tv.Timeout = time.Duration(runenv.IntParam("timeout_secs")) * time.Second
	}
	if runenv.IsParamSet("run_timeout_secs") {
		tv.RunTimeout = time.Duration(runenv.IntParam("run_timeout_secs")) * time.Second
	}
	if runenv.IsParamSet("leech_count") {
		tv.LeechCount = runenv.IntParam("leech_count")
	}
	if runenv.IsParamSet("passive_count") {
		tv.PassiveCount = runenv.IntParam("passive_count")
	}
	if runenv.IsParamSet("request_stagger") {
		tv.RequestStagger = time.Duration(runenv.IntParam("request_stagger")) * time.Millisecond
	}
	if runenv.IsParamSet("run_count") {
		tv.RunCount = runenv.IntParam("run_count")
	}
	if runenv.IsParamSet("max_connection_rate") {
		tv.MaxConnectionRate = runenv.IntParam("max_connection_rate")
	}
	if runenv.IsParamSet("enable_tcp") {
		tv.TCPEnabled = runenv.BooleanParam("enable_tcp")
	}
	if runenv.IsParamSet("seeder_rate") {
		tv.SeederRate = runenv.IntParam("seeder_rate")
	}
	if runenv.IsParamSet("enable_dht") {
		tv.DHTEnabled = runenv.BooleanParam("enable_dht")
	}
	if runenv.IsParamSet("long_lasting") {
		tv.LlEnabled = runenv.BooleanParam("long_lasting")
	}
	if runenv.IsParamSet("dialer") {
		tv.Dialer = runenv.StringParam("dialer")
	}
	if runenv.IsParamSet("number_waves") {
		tv.NumWaves = runenv.IntParam("number_waves")
	}

	bandwidths, err := utils.ParseIntArray(runenv.StringParam("bandwidth_mb"))
	if err != nil {
		return nil, err
	}
	latencies, err := utils.ParseIntArray(runenv.StringParam("latency_ms"))
	if err != nil {
		return nil, err
	}
	jitters, err := utils.ParseIntArray(runenv.StringParam("jitter_pct"))
	if err != nil {
		return nil, err
	}
	testFiles, err := utils.GetFileList(runenv)
	if err != nil {
		return nil, err
	}
	runenv.RecordMessage("Got file list: %v", testFiles)

	for _, f := range testFiles {
		for _, b := range bandwidths {
			for _, l := range latencies {
				latency := time.Duration(l) * time.Millisecond
				for _, j := range jitters {
					tv.Permutations = append(tv.Permutations, TestPermutation{File: f, Bandwidth: b, Latency: latency, JitterPct: j})
				}
			}
		}
	}

	return tv, nil
}

func InitializeTest(ctx context.Context, runenv *runtime.RunEnv, testvars *TestVars) (*TestData, error) {
	client := sync.MustBoundClient(ctx, runenv)
	nwClient := network.NewClient(client, runenv)

	nConfig, err := utils.GenerateAddrInfo(nwClient.MustGetDataNetworkIP().String())
	if err != nil {
		runenv.RecordMessage("Error generating node config")
		return nil, err
	}

	peers := sync.NewTopic("peers", &peer.AddrInfo{})

	// Get sequence number of this host
	seq, err := client.Publish(ctx, peers, *nConfig.AddrInfo)
	if err != nil {
		return nil, err
	}
	// Type of node and identifiers assigned.
	grpseq, nodetp, tpindex, err := parseType(ctx, runenv, client, nConfig.AddrInfo, seq)
	if err != nil {
		return nil, err
	}

	peerInfos := sync.NewTopic("peerInfos", &dialer.PeerInfo{})
	// Publish peer info for dialing
	_, err = client.Publish(ctx, peerInfos, &dialer.PeerInfo{Addr: *nConfig.AddrInfo, Nodetp: nodetp})
	if err != nil {
		return nil, err
	}

	var dialFn dialer.Dialer = dialer.DialOtherPeers
	if testvars.Dialer == "sparse" {
		dialFn = dialer.SparseDial
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
				return nil, err
			}
			// Sequence number starts from 1 (not 0)
			seedIndex = seedSeq - 1
		}
	}
	runenv.RecordMessage("Seed index %v for: %v", &nConfig.AddrInfo.ID, seedIndex)

	// Get addresses of all peers
	peerCh := make(chan *dialer.PeerInfo)
	sctx, cancelSub := context.WithCancel(ctx)
	if _, err := client.Subscribe(sctx, peerInfos, peerCh); err != nil {
		cancelSub()
		return nil, err
	}
	infos, err := dialer.PeerInfosFromChan(peerCh, runenv.TestInstanceCount)
	if err != nil {
		cancelSub()
		return nil, fmt.Errorf("no addrs in %d seconds", testvars.Timeout/time.Second)
	}
	cancelSub()
	runenv.RecordMessage("Got all addresses from other peers and network setup")

	/// --- Warm up

	// Signal that this node is in the given state, and wait for all peers to
	// send the same signal
	signalAndWaitForAll := func(state string) error {
		_, err := client.SignalAndWait(ctx, sync.State(state), runenv.TestInstanceCount)
		return err
	}

	return &TestData{client, nwClient,
		nConfig, infos, dialFn, signalAndWaitForAll,
		seq, grpseq, nodetp, tpindex, seedIndex}, nil
}

func (t *TestData) publishFile(ctx context.Context, fIndex int, cid *cid.Cid, runenv *runtime.RunEnv) error {
	// Create identifier for specific file size.
	rootCidTopic := getRootCidTopic(fIndex)

	runenv.RecordMessage("Published Added CID: %v", *cid)
	// Inform other nodes of the root CID
	if _, err := t.client.Publish(ctx, rootCidTopic, cid); err != nil {
		return fmt.Errorf("Failed to get Redis Sync rootCidTopic %w", err)
	}
	return nil
}

func (t *TestData) readFile(ctx context.Context, fIndex int, runenv *runtime.RunEnv, testvars *TestVars) (cid.Cid, error) {
	// Create identifier for specific file size.
	rootCidTopic := getRootCidTopic(fIndex)
	// Get the root CID from a seed
	rootCidCh := make(chan *cid.Cid, 1)
	sctx, cancelRootCidSub := context.WithCancel(ctx)
	defer cancelRootCidSub()
	if _, err := t.client.Subscribe(sctx, rootCidTopic, rootCidCh); err != nil {
		return cid.Undef, fmt.Errorf("Failed to subscribe to rootCidTopic %w", err)
	}
	// Note: only need to get the root CID from one seed - it should be the
	// same on all seeds (seed data is generated from repeatable random
	// sequence or existing file)
	rootCidPtr, ok := <-rootCidCh
	if !ok {
		return cid.Undef, fmt.Errorf("no root cid in %d seconds", testvars.Timeout/time.Second)
	}
	rootCid := *rootCidPtr
	runenv.RecordMessage("Received rootCid: %v", rootCid)
	return rootCid, nil
}

func (t *TestData) runTCPServer(ctx context.Context, fIndex int, f utils.TestFile, runenv *runtime.RunEnv, testvars *TestVars) error {
	// TCP variables
	tcpAddrTopic := getTCPAddrTopic(fIndex)
	runenv.RecordMessage("Starting TCP server in seed")

	// Start TCP server for file
	tcpServer, err := utils.SpawnTCPServer(ctx, t.nwClient.MustGetDataNetworkIP().String(), f)
	if err != nil {
		return fmt.Errorf("Failed to start tcpServer in seed %w", err)
	}
	// Inform other nodes of the TCPServerAddr
	runenv.RecordMessage("Publishing TCP address %v", tcpServer.Addr)
	if _, err = t.client.Publish(ctx, tcpAddrTopic, tcpServer.Addr); err != nil {
		return fmt.Errorf("Failed to get Redis Sync tcpAddr %w", err)
	}
	runenv.RecordMessage("Waiting to end finish TCP fetch")

	// Wait for all nodes to be done with TCP Fetch
	err = t.signalAndWaitForAll(fmt.Sprintf("tcp-fetch-%d", fIndex))
	if err != nil {
		return err
	}

	// At this point TCP interactions are finished.
	runenv.RecordMessage("Closing TCP server")
	tcpServer.Close()
	return nil
}

func (t *TestData) runTCPFetch(ctx context.Context, fIndex int, runenv *runtime.RunEnv, testvars *TestVars) (int64, error) {
	// TCP variables
	tcpAddrTopic := getTCPAddrTopic(fIndex)
	sctx, cancelTCPAddrSub := context.WithCancel(ctx)
	defer cancelTCPAddrSub()
	tcpAddrCh := make(chan *string, 1)
	if _, err := t.client.Subscribe(sctx, tcpAddrTopic, tcpAddrCh); err != nil {
		return 0, fmt.Errorf("Failed to subscribe to tcpServerTopic %w", err)
	}
	tcpAddrPtr, ok := <-tcpAddrCh

	runenv.RecordMessage("Received tcp server %v", tcpAddrPtr)
	if !ok {
		return 0, fmt.Errorf("no tcp server addr received in %d seconds", testvars.Timeout/time.Second)
	}
	runenv.RecordMessage("Start fetching a TCP file from seed")
	start := time.Now()
	utils.FetchFileTCP(*tcpAddrPtr)
	tcpFetch := time.Since(start).Nanoseconds()
	runenv.RecordMessage("Fetched TCP file after %d (ns)", tcpFetch)

	// Wait for all nodes to be done with TCP Fetch
	return tcpFetch, t.signalAndWaitForAll(fmt.Sprintf("tcp-fetch-%d", fIndex))
}

type NodeTestData struct {
	*TestData
	node utils.Node
}

func InitializeIPFSTest(ctx context.Context, runenv *runtime.RunEnv, testvars *TestVars) (*NodeTestData, error) {
	t, err := InitializeTest(ctx, runenv, testvars)
	if err != nil {
		return nil, err
	}
	// Create IPFS node
	runenv.RecordMessage("Preparing exchange for node: %v", testvars.ExchangeInterface)
	// Set exchange Interface
	exch, err := utils.SetExchange(ctx, testvars.ExchangeInterface)
	if err != nil {
		return nil, err
	}
	ipfsNode, err := utils.CreateIPFSNodeWithConfig(ctx, t.nConfig, exch, testvars.DHTEnabled)
	if err != nil {
		runenv.RecordFailure(err)
		return nil, err
	}

	err = t.signalAndWaitForAll("file-list-ready")
	if err != nil {
		return nil, err
	}

	return &NodeTestData{
		TestData: t,
		node:     ipfsNode,
	}, nil
}

func (t *NodeTestData) stillAlive(runenv *runtime.RunEnv, v *TestVars) {
	// starting liveness process for long-lasting experiments.
	if v.LlEnabled {
		go func(n utils.Node, runenv *runtime.RunEnv) {
			for {
				n.EmitKeepAlive(runenv)
				time.Sleep(15 * time.Second)
			}
		}(t.node, runenv)
	}
}

func (t *NodeTestData) addPublishFile(ctx context.Context, fIndex int, f utils.TestFile, runenv *runtime.RunEnv, testvars *TestVars) error {
	rate := float64(testvars.SeederRate) / 100
	seeders := runenv.TestInstanceCount - (testvars.LeechCount + testvars.PassiveCount)
	toSeed := int(math.Ceil(float64(seeders) * rate))

	// If this is the first run for this file size.
	// Only a rate of seeders add the file.
	if t.tpindex <= toSeed {
		// Generating and adding file to IPFS
		cid, err := generateAndAdd(ctx, runenv, t.node, f)
		if err != nil {
			return err
		}
		err = fractionalDAG(ctx, runenv, int(t.seedIndex), *cid, t.node.DAGService())
		if err != nil {
			return err
		}
		return t.publishFile(ctx, fIndex, cid, runenv)
	}
	return nil
}
func (t *NodeTestData) cleanupRun(ctx context.Context, runenv *runtime.RunEnv) error {
	// Disconnect peers
	for _, c := range t.node.Host().Network().Conns() {
		err := c.Close()
		if err != nil {
			return fmt.Errorf("Error disconnecting: %w", err)
		}
	}
	runenv.RecordMessage("Closed Connections")

	if t.nodetp == utils.Leech || t.nodetp == utils.Passive {
		// Clearing datastore
		// Also clean passive nodes so they don't store blocks from
		// previous runs.
		if err := t.node.ClearDatastore(ctx); err != nil {
			return fmt.Errorf("Error clearing datastore: %w", err)
		}
	}
	return nil
}

func (t *NodeTestData) cleanupFile(ctx context.Context) error {
	if t.nodetp == utils.Seed {
		// Between every file close the seed Node.
		// ipfsNode.Close()
		// runenv.RecordMessage("Closed Seed Node")
		if err := t.node.ClearDatastore(ctx); err != nil {
			return fmt.Errorf("Error clearing datastore: %w", err)
		}
	}
	return nil
}

func (t *NodeTestData) emitMetrics(runenv *runtime.RunEnv, runNum int,
	permutation TestPermutation, timeToFetch time.Duration, tcpFetch int64, leechFails int64,
	maxConnectionRate int) error {

	recorder := newMetricsRecorder(runenv, runNum, t.seq, t.grpseq, permutation.Latency, permutation.Bandwidth, int(permutation.File.Size()), t.nodetp, t.tpindex, maxConnectionRate)
	if t.nodetp == utils.Leech {
		recorder.Record("time_to_fetch", float64(timeToFetch))
		recorder.Record("leech_fails", float64(leechFails))
		recorder.Record("tcp_fetch", float64(tcpFetch))
	}

	return t.node.EmitMetrics(recorder)
}

func generateAndAdd(ctx context.Context, runenv *runtime.RunEnv, node utils.Node, f utils.TestFile) (*cid.Cid, error) {
	runenv.RecordMessage("Generating the new file in seeder")
	// Generate the file
	inputData := runenv.StringParam("input_data")
	runenv.RecordMessage("Starting to generate file for inputData: %s and file %v", inputData, f)
	tmpFile, err := f.GenerateFile()
	if err != nil {
		return nil, err
	}
	// runenv.RecordMessage("Adding the file to IPFS", tmpFile)
	// Add file to the IPFS network
	start := time.Now()
	cid, err := node.Add(ctx, tmpFile)
	end := time.Since(start).Milliseconds()
	if err != nil {
		runenv.RecordMessage("Error adding file to node: %w", err)
	}
	runenv.RecordMessage("Added to node %v in %d (ms)", cid, end)
	return &cid, err
}

func parseType(ctx context.Context, runenv *runtime.RunEnv, client *sync.DefaultClient, addrInfo *peer.AddrInfo, seq int64) (int64, utils.NodeType, int, error) {
	leechCount := runenv.IntParam("leech_count")
	passiveCount := runenv.IntParam("passive_count")

	grpCountOverride := false
	if runenv.TestGroupID != "" {
		grpLchLabel := runenv.TestGroupID + "_leech_count"
		if runenv.IsParamSet(grpLchLabel) {
			leechCount = runenv.IntParam(grpLchLabel)
			grpCountOverride = true
		}
		grpPsvLabel := runenv.TestGroupID + "_passive_count"
		if runenv.IsParamSet(grpPsvLabel) {
			passiveCount = runenv.IntParam(grpPsvLabel)
			grpCountOverride = true
		}
	}

	var nodetp utils.NodeType
	var tpindex int
	grpseq := seq
	seqstr := fmt.Sprintf("- seq %d / %d", seq, runenv.TestInstanceCount)
	grpPrefix := ""
	if grpCountOverride {
		grpPrefix = runenv.TestGroupID + " "

		var err error
		grpseq, err = getNodeSetSeq(ctx, client, addrInfo, runenv.TestGroupID)
		if err != nil {
			return grpseq, nodetp, tpindex, err
		}

		seqstr = fmt.Sprintf("%s (%d / %d of %s)", seqstr, grpseq, runenv.TestGroupInstanceCount, runenv.TestGroupID)
	}

	// Note: seq starts at 1 (not 0)
	switch {
	case grpseq <= int64(leechCount):
		nodetp = utils.Leech
		tpindex = int(grpseq) - 1
	case grpseq > int64(leechCount+passiveCount):
		nodetp = utils.Seed
		tpindex = int(grpseq) - 1 - (leechCount + passiveCount)
	default:
		nodetp = utils.Passive
		tpindex = int(grpseq) - 1 - leechCount
	}

	runenv.RecordMessage("I am %s %d %s", grpPrefix+nodetp.String(), tpindex, seqstr)

	return grpseq, nodetp, tpindex, nil
}

func getNodeSetSeq(ctx context.Context, client *sync.DefaultClient, addrInfo *peer.AddrInfo, setID string) (int64, error) {
	topic := sync.NewTopic("nodes"+setID, &peer.AddrInfo{})

	return client.Publish(ctx, topic, addrInfo)
}

func fractionalDAG(ctx context.Context, runenv *runtime.RunEnv, seedIndex int, c cid.Cid, dserv ipld.DAGService) error {

	//TODO: Explore this seed_fraction parameter.
	if !runenv.IsParamSet("seed_fraction") {
		return nil
	}
	seedFrac := runenv.StringParam("seed_fraction")
	if seedFrac == "" {
		return nil
	}

	parts := strings.Split(seedFrac, "/")
	if len(parts) != 2 {
		return fmt.Errorf("Invalid seed fraction %s", seedFrac)
	}
	numerator, nerr := strconv.ParseInt(parts[0], 10, 64)
	denominator, derr := strconv.ParseInt(parts[1], 10, 64)
	if nerr != nil || derr != nil {
		return fmt.Errorf("Invalid seed fraction %s", seedFrac)
	}

	ipldNode, err := dserv.Get(ctx, c)
	if err != nil {
		return err
	}

	nodes, err := getLeafNodes(ctx, ipldNode, dserv)
	if err != nil {
		return err
	}
	var del []cid.Cid
	for i := 0; i < len(nodes); i++ {
		idx := i + seedIndex
		if idx%int(denominator) >= int(numerator) {
			del = append(del, nodes[i].Cid())
		}
	}
	if err := dserv.RemoveMany(ctx, del); err != nil {
		return err
	}

	runenv.RecordMessage("Retained %d / %d of blocks from seed, removed %d / %d blocks", numerator, denominator, len(del), len(nodes))
	return nil
}

func getLeafNodes(ctx context.Context, node ipld.Node, dserv ipld.DAGService) ([]ipld.Node, error) {
	if len(node.Links()) == 0 {
		return []ipld.Node{node}, nil
	}

	var leaves []ipld.Node
	for _, l := range node.Links() {
		child, err := l.GetNode(ctx, dserv)
		if err != nil {
			return nil, err
		}
		childLeaves, err := getLeafNodes(ctx, child, dserv)
		if err != nil {
			return nil, err
		}
		leaves = append(leaves, childLeaves...)
	}

	return leaves, nil
}

func getRootCidTopic(id int) *sync.Topic {
	return sync.NewTopic(fmt.Sprintf("root-cid-%d", id), &cid.Cid{})
}

func getTCPAddrTopic(id int) *sync.Topic {
	return sync.NewTopic(fmt.Sprintf("tcp-addr-%d", id), "")
}

type metricsRecorder struct {
	runenv *runtime.RunEnv
	id     string
}

func newMetricsRecorder(runenv *runtime.RunEnv, runNum int, seq int64, grpseq int64,
	latency time.Duration, bandwidthMB int, fileSize int, nodetp utils.NodeType, tpindex int,
	maxConnectionRate int) utils.MetricsRecorder {

	latencyMS := latency.Milliseconds()
	instance := runenv.TestInstanceCount
	leechCount := runenv.IntParam("leech_count")
	passiveCount := runenv.IntParam("passive_count")

	id := fmt.Sprintf("topology:(%d-%d-%d)/maxConnectionRate:%d/latencyMS:%d/bandwidthMB:%d/run:%d/seq:%d/groupName:%s/groupSeq:%d/fileSize:%d/nodeType:%s/nodeTypeIndex:%d",
		instance-leechCount-passiveCount, leechCount, passiveCount, maxConnectionRate,
		latencyMS, bandwidthMB, runNum, seq, runenv.TestGroupID, grpseq, fileSize, nodetp, tpindex)

	return &metricsRecorder{runenv, id}
}

func (mr *metricsRecorder) Record(key string, value float64) {
	mr.runenv.R().RecordPoint(fmt.Sprintf("%s/name:%s", mr.id, key), value)
}
