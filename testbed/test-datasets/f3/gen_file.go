package main

import (
	"bytes"
	"io"
	"math/rand"
	"os"
)

// Script to create files of upto 5GB.
// Feel free to extend.

func main() {
	writeToDisk(SeededRandReader(1e9), "1G")
	writeToDisk(SeededRandReader(2e9), "2G")
	writeToDisk(SeededRandReader(3e9), "3G")
	writeToDisk(SeededRandReader(4e9), "4G")
	writeToDisk(SeededRandReader(5e9), "5G")
}

func writeToDisk(r io.Reader, fileName string) {
	f, err := os.Create(fileName)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	_, err = io.Copy(f, r)
	if err != nil {
		panic(err)
	}
}

func SeededRandReader(len uint64) io.Reader {
	randReader := rand.New(rand.NewSource(int64(len)))
	data := make([]byte, len)
	randReader.Read(data)
	return bytes.NewReader(data)
}
