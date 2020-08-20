// tut_tcpclient_filereceiver project main.go
// Made by Gilles Van Vlasselaer
// More info about it on www.mrwaggel.be/post/golang-sending-a-file-over-tcp/

package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

//Define that the binairy data of the file will be sent 1024 bytes at a time
const BUFFERSIZE = 1024

func main() {
	connection, err := net.Dial("tcp", "localhost:27001")
	if err != nil {
		panic(err)
	}
	defer connection.Close()
	fmt.Println("Connected to server, start receiving the file name and file size")
	//Create buffer to read in the name and size of the file
	bufferFileName := make([]byte, 64)
	bufferFileSize := make([]byte, 10)
	//Get the filesize
	connection.Read(bufferFileSize)
	//Strip the ':' from the received size, convert it to a int64
	fileSize, _ := strconv.ParseInt(strings.Trim(string(bufferFileSize), ":"), 10, 64)
	//Get the filename
	connection.Read(bufferFileName)
	//Strip the ':' once again but from the received file name now
	fileName := strings.Trim(string(bufferFileName), ":")
	//Create a new file to write in
	newFile, err := os.Create(fileName)
	if err != nil {
		panic(err)
	}
	defer newFile.Close()
	//Create a variable to store in the total amount of data that we received already
	var receivedBytes int64
	//Start writing in the file
	for {
		if (fileSize - receivedBytes) < BUFFERSIZE {
			io.CopyN(newFile, connection, (fileSize - receivedBytes))
			//Empty the remaining bytes that we don't need from the network buffer
			connection.Read(make([]byte, (receivedBytes+BUFFERSIZE)-fileSize))
			//We are done writing the file, break out of the loop
			break
		}
		io.CopyN(newFile, connection, BUFFERSIZE)
		//Increment the counter
		receivedBytes += BUFFERSIZE
	}
	fmt.Println("Received file completely!")
}
