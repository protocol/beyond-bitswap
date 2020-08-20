// tut_tcpserver_filesend project main.go
// Made by Gilles Van Vlasselaer
// More info about it on www.mrwaggel.be/post/golang-sending-a-file-over-tcp/

package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
)

//Define the size of how big the chunks of data will be send each time
const BUFFERSIZE = 1024

func main() {
	//Create a TCP listener on localhost with porth 27001
	server, err := net.Listen("tcp", "localhost:27001")
	if err != nil {
		fmt.Println("Error listetning: ", err)
		os.Exit(1)
	}
	defer server.Close()
	fmt.Println("Server started! Waiting for connections...")
	//Spawn a new goroutine whenever a client connects
	for {
		connection, err := server.Accept()
		if err != nil {
			fmt.Println("Error: ", err)
			os.Exit(1)
		}
		fmt.Println("Client connected")
		go sendFileToClient(connection)
	}
}

//This function is to 'fill'
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

func sendFileToClient(connection net.Conn) {
	fmt.Println("A client has connected!")
	defer connection.Close()
	//Open the file that needs to be send to the client
	file, err := os.Open("dummyfile.dat")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer file.Close()
	//Get the filename and filesize
	fileInfo, err := file.Stat()
	if err != nil {
		fmt.Println(err)
		return
	}
	fileSize := fillString(strconv.FormatInt(fileInfo.Size(), 10), 10)
	fileName := fillString(fileInfo.Name(), 64)
	//Send the file header first so the client knows the filename and how long it has to read the incomming file
	fmt.Println("Sending filename and filesize!")
	//Write first 10 bytes to client telling them the filesize
	connection.Write([]byte(fileSize))
	//Write 64 bytes to client containing the filename
	connection.Write([]byte(fileName))
	//Initialize a buffer for reading parts of the file in
	sendBuffer := make([]byte, BUFFERSIZE)
	//Start sending the file to the client
	fmt.Println("Start sending file!")
	for {
		_, err = file.Read(sendBuffer)
		if err == io.EOF {
			//End of file reached, break out of for loop
			break
		}
		connection.Write(sendBuffer)
	}
	fmt.Println("File has been sent, closing connection!")
	return
}
