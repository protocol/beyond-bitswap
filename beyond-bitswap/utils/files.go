package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"

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
		randReader = rand.New(rand.NewSource(2))
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

func (n *IPFSNode) Add(ctx context.Context, runenv *runtime.RunEnv, f InputFile) (path.Resolved, error) {
	inputData := runenv.StringParam("input_data")
	var tmpFile files.File
	// We need to specify how we generate the data for every case.
	runenv.RecordMessage("Starting to add file for inputData: %s and file %v", inputData, f)
	if inputData == "random" {
		tmpFile = files.NewReaderFile(RandReader(int(f.Size)))
	} else {
		var err error
		tmpFile, err = getUnixfsFile(f.Path)
		if err != nil {
			return nil, err
		}
	}
	cid, err := n.API.Unixfs().Add(ctx, tmpFile)
	runenv.RecordMessage("Added to network %v", cid)

	return cid, err
}

func getUnixfsFile(path string) (files.File, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	st, err := file.Stat()
	if err != nil {
		return nil, err
	}

	f, err := files.NewReaderPathFile(path, file, st)
	if err != nil {
		return nil, err
	}

	return f, nil
}
