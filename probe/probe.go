package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	logging "github.com/ipfs/go-log"
	"github.com/ipfs/interface-go-ipfs-core/path"
	// This package is needed so that all the preloaded plugins are loaded automatically
	// bsnet "github.com/ipfs/go-bitswap/network"
)

// Process commands received from prompt
func processInput(ctx context.Context, ipfs *IPFSNode, text string, done chan bool) error {
	text = strings.ReplaceAll(text, "\n", "")
	text = strings.ReplaceAll(text, " ", "")
	words := strings.Split(text, "_")

	// Defer notifying the that processing is finished.
	defer func() {
		done <- true
	}()

	if words[0] == "exit" {
		os.Exit(0)
	}
	if words[0] == "disconnect" {
		for _, c := range ipfs.Node.PeerHost.Network().Conns() {
			err := c.Close()
			if err != nil {
				return fmt.Errorf("Error disconnecting: %w", err)
			}
			fmt.Println("Disconnected from every peer")
			return nil
		}
	}
	if len(words) < 2 {
		fmt.Println("Wrong number of arguments")
		return fmt.Errorf("Wrong number of arguments")
	}
	// If we use add we can add random content to the network.
	if words[0] == "add" {
		size, err := strconv.Atoi(words[1])
		if err != nil {
			fmt.Println("Not a valid size for random add")
			return err
		}
		addRandomContent(ctx, ipfs, size)
	} else if words[0] == "connect" {
		connectPeer(ctx, ipfs, words[1])
	} else if words[0] == "addFile" {
		addFile(ctx, ipfs, words[1])
	} else if words[0] == "get" {
		fPath := path.New(words[1])
		err := getContent(ctx, ipfs, fPath, false)
		if err != nil {
			fmt.Println("Couldn't find content", err)
			return err
		}
	} else if words[0] == "pin" {
		fPath := path.New(words[1])
		err := getContent(ctx, ipfs, fPath, true)
		if err != nil {
			fmt.Println("Couldn't find content", err)
			return err
		}
	} else {
		fmt.Println("[!] Wrong command! Only available add, addFile, pin, get, connect, exit")
	}
	return nil
}

func main() {
	addDirectory := flag.String("addDirectory", "", "Add a directory to the probe")
	debug := flag.Bool("debug", false, "Set debug logging")

	flag.Parse()
	if *debug {
		logging.SetLogLevel("bitswap", "DEBUG")
		logging.SetLogLevel("bitswap_network", "DEBUG")
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("-- Getting an IPFS node running -- ")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := setupPlugins(""); err != nil {
		panic(fmt.Errorf("Failed setting up plugins: %s", err))
	}

	// Spawn a node using a temporary path, creating a temporary repo for the run
	fmt.Println("Spawning node on a temporary repo")
	ipfs, err := CreateIPFSNode(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to spawn ephemeral node: %s", err))
	}

	// Adding random content for testing.
	addRandomContent(ctx, ipfs, 11111)
	if *addDirectory != "" {
		// Adding directory,
		fmt.Println("Adding inputData directory")
		err := addFile(ctx, ipfs, *addDirectory)
		if err != nil {
			panic("Wrong directory")
		}
	}

	ch := make(chan string)
	chSignal := make(chan os.Signal)
	done := make(chan bool)
	signal.Notify(chSignal, os.Interrupt, syscall.SIGTERM)

	// Prompt routine
	go func(ch chan string, done chan bool) {
		for {
			fmt.Print(">> Enter command: ")
			text, _ := reader.ReadString('\n')
			ch <- text
			<-done
		}
	}(ch, done)

	// Processing loop.
	for {
		select {
		case text := <-ch:
			processInput(ctx, ipfs, text, done)

		case <-chSignal:
			fmt.Printf("\nUse exit to close the tool\n")
			fmt.Printf(">> Enter command: ")

		}
	}
}
