package test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/adlrocha/beyond-bitswap/testbed/utils"
	"github.com/adlrocha/beyond-bitswap/testbed/utils/dialer"

	"github.com/testground/sdk-go/network"
)

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
}

type TestData struct {
	client              *sync.DefaultClient
	nwClient            *network.Client
	ipfsNode            *utils.IPFSNode
	testFiles           []utils.TestFile
	nConfig             *utils.NodeConfig
	peerInfos           []dialer.PeerInfo
	dialFn              dialer.Dialer
	latency             time.Duration
	bandwidth           int
	signalAndWaitForAll func(state string) error
	seq                 int64
	grpseq              int64
	nodetp              utils.NodeType
	tpindex             int
}

func getEnvVars(runenv *runtime.RunEnv) *TestVars {
	return &TestVars{
		ExchangeInterface: runenv.StringParam("exchange_interface"),
		Timeout:           time.Duration(runenv.IntParam("timeout_secs")) * time.Second,
		RunTimeout:        time.Duration(runenv.IntParam("run_timeout_secs")) * time.Second,
		LeechCount:        runenv.IntParam("leech_count"),
		PassiveCount:      runenv.IntParam("passive_count"),
		RequestStagger:    time.Duration(runenv.IntParam("request_stagger")) * time.Millisecond,
		RunCount:          runenv.IntParam("run_count"),
		MaxConnectionRate: runenv.IntParam("max_connection_rate"),
		TCPEnabled:        runenv.BooleanParam("enable_tcp"),
		SeederRate:        runenv.IntParam("seeder_rate"),
		DHTEnabled:        runenv.BooleanParam("enable_dht"),
		LlEnabled:         runenv.BooleanParam("long_lasting"),
		Dialer:            runenv.StringParam("dialer"),
		NumWaves:          runenv.IntParam("number_waves"),
	}
}

func InitializeTest(ctx context.Context, runenv *runtime.RunEnv, testvars *TestVars) (*TestData, error) {
	client := sync.MustBoundClient(ctx, runenv)
	nwClient := network.NewClient(client, runenv)

	runenv.RecordMessage("Preparing exchange for node: %v", testvars.ExchangeInterface)
	// Set exchange Interface
	exch, err := utils.SetExchange(ctx, testvars.ExchangeInterface)
	if err != nil {
		return nil, err
	}
	nConfig, err := utils.GenerateAddrInfo(nwClient.MustGetDataNetworkIP().String())
	if err != nil {
		runenv.RecordMessage("Error generating node config")
		return nil, err
	}
	// Create IPFS node
	ipfsNode, err := utils.CreateIPFSNodeWithConfig(ctx, nConfig, exch, testvars.DHTEnabled)
	// ipfsNode, err := utils.CreateIPFSNode(ctx)
	if err != nil {
		runenv.RecordFailure(err)
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

	// Set up network (with traffic shaping)
	latency, bandwidthMB, err := utils.SetupNetwork(ctx, runenv, nwClient, nodetp, tpindex)
	if err != nil {
		return nil, fmt.Errorf("Failed to set up network: %v", err)
	}

	// According to the input data get the file size or the files to add.
	testFiles, err := utils.GetFileList(runenv)
	if err != nil {
		return nil, err
	}
	runenv.RecordMessage("Got file list: %v", testFiles)

	// Signal that this node is in the given state, and wait for all peers to
	// send the same signal
	signalAndWaitForAll := func(state string) error {
		_, err := client.SignalAndWait(ctx, sync.State(state), runenv.TestInstanceCount)
		return err
	}

	err = signalAndWaitForAll("file-list-ready")
	if err != nil {
		return nil, err
	}

	return &TestData{client, nwClient, ipfsNode, testFiles,
		nConfig, infos, dialFn,
		latency, bandwidthMB, signalAndWaitForAll,
		seq, grpseq, nodetp, tpindex}, nil
}

func (t *TestData) stillAlive(runenv *runtime.RunEnv, v *TestVars) {
	// starting liveness process for long-lasting experiments.
	if v.LlEnabled {
		go func(n *utils.IPFSNode, runenv *runtime.RunEnv) {
			for {
				runenv.RecordMessage("I am still alive! Total In: %d - TotalOut: %d",
					t.ipfsNode.Node.Reporter.GetBandwidthTotals().TotalIn,
					t.ipfsNode.Node.Reporter.GetBandwidthTotals().TotalOut)
				time.Sleep(15 * time.Second)
			}
		}(t.ipfsNode, runenv)
	}

}

func generateAndAdd(ctx context.Context, runenv *runtime.RunEnv, ipfsNode *utils.IPFSNode, f utils.TestFile) (*cid.Cid, error) {
	runenv.RecordMessage("Generating the new file in seeder")
	// Generate the file
	tmpFile, err := ipfsNode.GenerateFile(ctx, runenv, f)
	if err != nil {
		return nil, err
	}
	// runenv.RecordMessage("Adding the file to IPFS", tmpFile)
	// Add file to the IPFS network
	cidFile, err := ipfsNode.Add(ctx, runenv, tmpFile)
	if err != nil {
		runenv.RecordMessage("Error adding file to IPFS %w", err)
		return nil, err
	}
	cid := cidFile.Cid()
	return &cid, nil
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

func setupSeed(ctx context.Context, runenv *runtime.RunEnv, node *utils.Node, fileSize int, seedIndex int) (cid.Cid, error) {
	tmpFile := utils.RandReader(fileSize)
	ipldNode, err := node.Add(ctx, tmpFile)
	if err != nil {
		return cid.Cid{}, err
	}

	//TODO: Explore this seed_fraction parameter.
	if !runenv.IsParamSet("seed_fraction") {
		return ipldNode.Cid(), nil
	}
	seedFrac := runenv.StringParam("seed_fraction")
	if seedFrac == "" {
		return ipldNode.Cid(), nil
	}

	parts := strings.Split(seedFrac, "/")
	if len(parts) != 2 {
		return cid.Cid{}, fmt.Errorf("Invalid seed fraction %s", seedFrac)
	}
	numerator, nerr := strconv.ParseInt(parts[0], 10, 64)
	denominator, derr := strconv.ParseInt(parts[1], 10, 64)
	if nerr != nil || derr != nil {
		return cid.Cid{}, fmt.Errorf("Invalid seed fraction %s", seedFrac)
	}

	nodes, err := getLeafNodes(ctx, ipldNode, node.Dserv)
	if err != nil {
		return cid.Cid{}, err
	}
	var del []cid.Cid
	for i := 0; i < len(nodes); i++ {
		idx := i + seedIndex
		if idx%int(denominator) >= int(numerator) {
			del = append(del, nodes[i].Cid())
		}
	}
	if err := node.Dserv.RemoveMany(ctx, del); err != nil {
		return cid.Cid{}, err
	}

	runenv.RecordMessage("Retained %d / %d of blocks from seed, removed %d / %d blocks", numerator, denominator, len(del), len(nodes))
	return ipldNode.Cid(), nil
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

func emitMetrics(runenv *runtime.RunEnv, bsnode *utils.Node, runNum int, seq int64, grpseq int64,
	latency time.Duration, bandwidthMB int, fileSize int, nodetp utils.NodeType, tpindex int, timeToFetch time.Duration) error {

	stats, err := bsnode.Bitswap.Stat()
	if err != nil {
		return fmt.Errorf("Error getting stats from Bitswap: %w", err)
	}

	latencyMS := latency.Milliseconds()
	instance := runenv.TestInstanceCount
	leechCount := runenv.IntParam("leech_count")
	passiveCount := runenv.IntParam("passive_count")

	id := fmt.Sprintf("topology:(%d-%d-%d)/latencyMS:%d/bandwidthMB:%d/run:%d/seq:%d/groupName:%s/groupSeq:%d/fileSize:%d/nodeType:%s/nodeTypeIndex:%d",
		instance-leechCount-passiveCount, leechCount, passiveCount,
		latencyMS, bandwidthMB, runNum, seq, runenv.TestGroupID, grpseq, fileSize, nodetp, tpindex)

	if nodetp == utils.Leech {
		runenv.R().RecordPoint(fmt.Sprintf("%s/name:time_to_fetch", id), float64(timeToFetch))
		// runenv.R().RecordPoint(fmt.Sprintf("%s/name:num_dht", id), float64(stats.NumDHT))
	}
	runenv.R().RecordPoint(fmt.Sprintf("%s/name:msgs_rcvd", id), float64(stats.MessagesReceived))
	runenv.R().RecordPoint(fmt.Sprintf("%s/name:data_sent", id), float64(stats.DataSent))
	runenv.R().RecordPoint(fmt.Sprintf("%s/name:data_rcvd", id), float64(stats.DataReceived))
	runenv.R().RecordPoint(fmt.Sprintf("%s/name:block_data_rcvd", id), float64(stats.BlockDataReceived))
	runenv.R().RecordPoint(fmt.Sprintf("%s/name:dup_data_rcvd", id), float64(stats.DupDataReceived))
	runenv.R().RecordPoint(fmt.Sprintf("%s/name:blks_sent", id), float64(stats.BlocksSent))
	runenv.R().RecordPoint(fmt.Sprintf("%s/name:blks_rcvd", id), float64(stats.BlocksReceived))
	runenv.R().RecordPoint(fmt.Sprintf("%s/name:dup_blks_rcvd", id), float64(stats.DupBlksReceived))

	return nil
}
