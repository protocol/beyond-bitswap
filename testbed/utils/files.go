package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/testground/sdk-go/runtime"
)

// var randReader *rand.Rand

// TestFile interface for input files used.
type TestFile interface {
	GenerateFile() (files.Node, error)
	Size() int64
}

// RandFile represents a randomly generated file
type RandFile struct {
	size int64
}

// PathFile is a generated from file.
type PathFile struct {
	Path  string
	size  int64
	isDir bool
}

// GenerateFile generates new randomly generated file
func (f *RandFile) GenerateFile() (files.Node, error) {
	return files.NewReaderFile(RandReader(int(f.size))), nil
}

// Size returns size
func (f *RandFile) Size() int64 {
	return f.size
}

// Size returns size
func (f *PathFile) Size() int64 {
	return f.size
}

// GenerateFile gets the file from path
func (f *PathFile) GenerateFile() (files.Node, error) {
	tmpFile, err := getUnixfsNode(f.Path)
	if err != nil {
		return nil, err
	}
	return tmpFile, nil
}

// RandFromReader Generates random file from existing reader
func RandFromReader(randReader *rand.Rand, len int) io.Reader {
	if randReader == nil {
		randReader = rand.New(rand.NewSource(2))
	}
	data := make([]byte, len)
	randReader.Read(data)
	return bytes.NewReader(data)
}

// DirSize computes total size of the of the direcotry.
func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}

// RandReader generates random data from seed.
func RandReader(len int) io.Reader {
	randReader := rand.New(rand.NewSource(time.Now().Unix()))
	data := make([]byte, len)
	randReader.Read(data)
	return bytes.NewReader(data)
}

func GetFileList(runenv *runtime.RunEnv) ([]TestFile, error) {
	listFiles := []TestFile{}
	inputData := runenv.StringParam("input_data")

	switch inputData {
	case "files":
		path := runenv.StringParam("data_dir")
		runenv.RecordMessage("Getting file list for %s", path)
		files, err := ioutil.ReadDir(path)
		if err != nil {
			return nil, err
		}

		for _, file := range files {
			var size int64

			// Assign the right size.
			if file.IsDir() {
				size, err = dirSize(path + "/" + file.Name())
				if err != nil {
					return nil, err
				}
			} else {
				size = file.Size()
			}

			// Append the file.
			listFiles = append(listFiles,
				&PathFile{
					Path:  path + "/" + file.Name(),
					size:  size,
					isDir: file.IsDir()})
		}
		return listFiles, nil
	case "random":
		fileSizes, err := ParseIntArray(runenv.StringParam("file_size"))
		runenv.RecordMessage("Getting file list for random with sizes: %v", fileSizes)
		if err != nil {
			return nil, err
		}
		for _, v := range fileSizes {
			listFiles = append(listFiles, &RandFile{size: int64(v)})
		}
		return listFiles, nil
	case "custom":
		return nil, fmt.Errorf("Custom inputData not implemented yet")
	default:
		return nil, fmt.Errorf("Inputdata type not implemented")
	}
}

func (n *IPFSNode) GenerateFile(ctx context.Context, runenv *runtime.RunEnv, f TestFile) (files.Node, error) {
	inputData := runenv.StringParam("input_data")
	runenv.RecordMessage("Starting to generate file for inputData: %s and file %v", inputData, f)
	tmpFile, err := f.GenerateFile()
	if err != nil {
		return nil, err
	}
	return tmpFile, nil
}

func (n *IPFSNode) Add(ctx context.Context, runenv *runtime.RunEnv, tmpFile files.Node) (path.Resolved, error) {
	cid, err := n.API.Unixfs().Add(ctx, tmpFile)
	runenv.RecordMessage("Added to network %v", cid)
	return cid, err
}

func getUnixfsNode(path string) (files.Node, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	f, err := files.NewSerialFile(path, false, st)
	if err != nil {
		return nil, err
	}

	return f, nil
}
