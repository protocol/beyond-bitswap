package utils

import (
	"context"
	"strings"
	"time"

	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
)

// SetupNetwork instructs the sidecar (if enabled) to setup the network for this
// test case.
func SetupNetwork(ctx context.Context, runenv *runtime.RunEnv,
	nwClient *network.Client, nodetp NodeType, tpindex int, baseLatency time.Duration,
	bandwidth int, jitterPct int) error {

	if !runenv.TestSidecar {
		return nil
	}

	// Wait for the network to be initialized.
	if err := nwClient.WaitNetworkInitialized(ctx); err != nil {
		return err
	}

	latency, err := getLatency(runenv, nodetp, tpindex, baseLatency)
	if err != nil {
		return err
	}

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

	runenv.RecordMessage("%s %d has %s latency (%d%% jitter) and %dMB bandwidth", nodetp, tpindex, latency, jitterPct, bandwidth)

	return nwClient.ConfigureNetwork(ctx, cfg)
}

// If there's a latency specific to the node type, overwrite the default latency
func getLatency(runenv *runtime.RunEnv, nodetp NodeType, tpindex int, baseLatency time.Duration) (time.Duration, error) {
	if nodetp == Seed {
		return getTypeLatency(runenv, "seed_latency_ms", tpindex, baseLatency)
	} else if nodetp == Leech {
		return getTypeLatency(runenv, "leech_latency_ms", tpindex, baseLatency)
	}
	return baseLatency, nil
}

// If the parameter is a comma-separated list, each value in the list
// corresponds to the type index. For example:
// seed_latency_ms=100,200,400
// means that
// - the first seed has 100ms latency
// - the second seed has 200ms latency
// - the third seed has 400ms latency
// - any subsequent seeds have defaultLatency
func getTypeLatency(runenv *runtime.RunEnv, param string, tpindex int, baseLatency time.Duration) (time.Duration, error) {
	// No type specific latency set, just return the default
	if !runenv.IsParamSet(param) {
		return baseLatency, nil
	}

	// Not a comma-separated list, interpret the value as an int and apply
	// the same latency to all peers of this type
	if !strings.Contains(runenv.StringParam(param), ",") {
		return baseLatency + time.Duration(runenv.IntParam(param)) * time.Millisecond, nil
	}

	// Comma separated list, the position in the list corresponds to the
	// type index
	latencies, err := ParseIntArray(runenv.StringParam(param))
	if err != nil {
		return 0, err
	}
	if tpindex < len(latencies) {
		return baseLatency + time.Duration(latencies[tpindex]) * time.Millisecond, nil
	}

	// More peers of this type than entries in the list. Return the default
	// latency for peers not covered by list entries
	return baseLatency, nil
}
