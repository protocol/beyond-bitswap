package utils

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"strconv"
	"strings"
	"sync"

	files "github.com/ipfs/go-ipfs-files"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/testground/sdk-go/runtime"
)

// TCPServer structure
type TCPServer struct {
	quit     chan interface{}
	listener net.Listener
	file     TestFile
	Addr     string
	wg       sync.WaitGroup
}

// SpawnTCPServer Spawns a TCP server that serves a specific file.
func SpawnTCPServer(ctx context.Context, ip string, tmpFile TestFile) (*TCPServer, error) {
	//Create a TCP istener on localhost with porth 27001
	listener, err := net.Listen("tcp", ip+":0")
	fmt.Println("listening at: ", listener.Addr().String())
	if err != nil {
		fmt.Println("Error listetning: ", err)
		return nil, err
	}
	//Spawn a new goroutine whenever a client connects
	s := &TCPServer{
		quit:     make(chan interface{}),
		listener: listener,
		file:     tmpFile,
		Addr:     listener.Addr().String(),
	}
	s.wg.Add(1)
	go s.Start()
	return s, nil
}

// Start listening for conections.
func (s *TCPServer) Start() {
	// Start listening routine
	defer s.wg.Done()
	for {
		connection, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.quit:
				return
			default:
				fmt.Println("Accept error", err)
			}
		} else {
			s.wg.Add(1)
			go s.sendFileToClient(connection)
			s.wg.Done()
		}
	}
}

// Close the TCP Server.
func (s *TCPServer) Close() {
	close(s.quit)
	s.listener.Close()
	s.wg.Wait()
	fmt.Println("Successfully closed TCP server")
}

// Format for fileSize
func fillString(returnString string, toLength int) string {
	for {
		lengtString := len(returnString)
		if lengtString < toLength {
			returnString = returnString + ":"
			continue
		}
		break
	}
	return returnString
}

// Sends file to client.
func (s *TCPServer) sendFileToClient(connection net.Conn) {
	defer connection.Close()
	// Passing files.Node directly produced that routines
	// concurrently accessed their reader. Instead of sending the
	// file n times, each client received a part.
	tmpFile, err := s.file.GenerateFile()
	if err != nil {
		fmt.Println("Failed generating file:", err)
		return
	}

	var f io.Reader
	f = files.ToFile(tmpFile)
	if f == nil {
		d := files.ToDir(tmpFile)
		if d == nil {
			fmt.Println("Must be a file or dir")
			return
		}
		f = files.NewMultiFileReader(d, false)
	}

	size := s.file.Size()
	// The first write is to notify the size.
	fileSize := fillString(strconv.FormatInt(size, 10), 10)
	fmt.Println("Sending file of: ", size)
	connection.Write([]byte(fileSize))

	// Sending the file.
	buf := make([]byte, network.MessageSizeMax)
	written, err := io.CopyBuffer(connection, f, buf)
	if err != nil {
		log.Fatal(err)
	}
	connection.Close()

	fmt.Println("Bytes sent from server", written)
	return
}

// FetchFileTCP fetchs the file server in an address by a TCP server.
func FetchFileTCP(connection net.Conn, runEnv *runtime.RunEnv) {
	// read file size
	bufferFileSize := make([]byte, 10)
	if _, err := connection.Read(bufferFileSize); err != nil {
		runEnv.RecordFailure(err)
		return
	}
	fileSize, _ := strconv.ParseInt(strings.Trim(string(bufferFileSize), ":"), 10, 64)

	// Read from connection
	buf := make([]byte, network.MessageSizeMax)
	w, err := io.CopyBuffer(ioutil.Discard, connection, buf)
	if err != nil {
		runEnv.RecordFailure(err)
		return
	}
	if w != fileSize {
		runEnv.RecordFailure(fmt.Errorf("expcted:%d, got: %d bytes", fileSize, w))
	}
}
