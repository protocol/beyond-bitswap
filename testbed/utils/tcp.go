package utils

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	files "github.com/ipfs/go-ipfs-files"
)

//Define the size of how big the chunks of data will be send each time
const BUFFERSIZE = 1024

// TCPServer structure
type TCPServer struct {
	quit     chan interface{}
	listener net.Listener
	file     files.Node
	Addr     string
	wg       sync.WaitGroup
}

// SpawnTCPServer Spawns a TCP server that serves a specific file.
func SpawnTCPServer(ctx context.Context, ip string, tmpFile files.Node) (*TCPServer, error) {
	//Create a TCP istener on localhost with porth 27001
	listener, err := net.Listen("tcp", ip+":0")
	fmt.Println("listening at: ", listener.Addr().String())
	if err != nil {
		fmt.Println("Error listetning: ", err)
		return nil, err
	}
	//Spawn a new goroutine whenever a lient connects
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
func fillString(retunString string, toLength int) string {
	for {
		lengtString := len(retunString)
		if lengtString < toLength {
			retunString = retunString + ":"
			continue
		}
		break
	}
	return retunString
}

// Sends file to client.
func (s *TCPServer) sendFileToClient(connection net.Conn) {
	defer connection.Close()
	sendBuffer := make([]byte, BUFFERSIZE)
	size, _ := s.file.Size()
	// The first buffer is to notify the size.
	fileSize := fillString(strconv.FormatInt(size, 10), 10)
	connection.Write([]byte(fileSize))
	for {
		_, err := files.ToFile(s.file).Read(sendBuffer)
		if err == io.EOF {
			break
		}
		connection.Write(sendBuffer)
	}
	return
}

// FetchFileTCP fetchs the file server in an address by a TCP server.
func FetchFileTCP(addr string) {
	connection, err := net.Dial("tcp", addr)
	if err != nil {
		panic(err)
	}
	defer connection.Close()

	bufferFileSize := make([]byte, 10)
	connection.Read(bufferFileSize)
	fileSize, _ := strconv.ParseInt(strings.Trim(string(bufferFileSize), ":"), 10, 64)

	newFile, err := os.OpenFile("/dev/null", os.O_WRONLY, 0)

	if err != nil {
		panic(err)
	}
	defer newFile.Close()
	var receivedBytes int64

	for {
		if (fileSize - receivedBytes) < BUFFERSIZE {
			io.CopyN(newFile, connection, (fileSize - receivedBytes))
			connection.Read(make([]byte, (receivedBytes+BUFFERSIZE)-fileSize))
			receivedBytes = fileSize - receivedBytes
			break
		}
		io.CopyN(newFile, connection, BUFFERSIZE)
		receivedBytes += BUFFERSIZE
	}
	fmt.Println("Finished fetch..")
}
