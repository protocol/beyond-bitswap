package utils

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	files "github.com/ipfs/go-ipfs-files"
	logging "github.com/ipfs/go-log/v2"
	"github.com/testground/sdk-go/runtime"
)

var log = logging.Logger("utils")

// var randReader *rand.Rand

// TestFile interface for input files used.
type TestFile interface {
	GenerateFile() (files.Node, error)
	Size() int64
}

// RandFile represents a randomly generated file
type RandFile struct {
	size int64
	seed int64
}

// PathFile is a generated from file.
type PathFile struct {
	Path  string
	size  int64
	isDir bool
}

// GenerateFile generates new randomly generated file
func (f *RandFile) GenerateFile() (files.Node, error) {
	r := SeededRandReader(int(f.size), f.seed)

	path := fmt.Sprintf("/tmp-%d", rand.Uint64())
	tf, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(tf, r); err != nil {
		return nil, err
	}
	if err := tf.Close(); err != nil {
		return nil, err
	}

	return getUnixfsNode(path)
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
func SeededRandReader(len int, seed int64) io.Reader {
	randReader := rand.New(rand.NewSource(seed))
	data := make([]byte, len)
	randReader.Read(data)
	return bytes.NewReader(data)
}

// RandReader generates random data randomly.
func RandReader(len int) io.Reader {
	return SeededRandReader(len, time.Now().Unix())
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
			if file.Name() == ".gitkeep" {
				continue
			}
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
		for i, v := range fileSizes {
			listFiles = append(listFiles, &RandFile{size: int64(v), seed: int64(i)})
		}
		return listFiles, nil
	case "custom":
		return nil, fmt.Errorf("Custom inputData not implemented yet")
	default:
		return nil, fmt.Errorf("Inputdata type not implemented")
	}
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
