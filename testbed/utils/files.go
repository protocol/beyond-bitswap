package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"time"

	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/testground/sdk-go/runtime"
)

var randReader *rand.Rand

type InputFile struct {
	Path  string
	Size  int64
	isDir bool
}

func RandReader(len int) io.Reader {
	if randReader == nil {
		randReader = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	data := make([]byte, len)
	randReader.Read(data)
	return bytes.NewReader(data)
}

func GetFileList(runenv *runtime.RunEnv) ([]InputFile, error) {
	listFiles := []InputFile{}
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
			listFiles = append(listFiles,
				InputFile{
					Path:  path + "/" + file.Name(),
					Size:  file.Size(),
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
			listFiles = append(listFiles, InputFile{Size: int64(v)})
		}
		return listFiles, nil
	case "custom":
		return nil, fmt.Errorf("Custom inputData not implemented yet")
	default:
		return nil, fmt.Errorf("Inputdata type not implemented")
	}
}

func (n *IPFSNode) GenerateFile(ctx context.Context, runenv *runtime.RunEnv, f InputFile) (files.Node, error) {
	inputData := runenv.StringParam("input_data")
	var tmpFile files.Node
	var err error

	// We need to specify how we generate the data for every case.
	runenv.RecordMessage("Starting to generate file for inputData: %s and file %v", inputData, f)
	if inputData == "random" {
		tmpFile = files.NewReaderFile(RandReader(int(f.Size)))
	} else {
		tmpFile, err = getUnixfsNode(f.Path)
		if err != nil {
			return nil, err
		}
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
