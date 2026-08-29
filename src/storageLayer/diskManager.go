package storageLayer

import (
	"errors"
	"os"
	"sync"
)

type DiskManager struct {
	file          *os.File
	mu            sync.Mutex
	numberOfPages uint16
}

var errorPageNotFound = errors.New("page not found")

func openDiskManager(path string) (*DiskManager, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)

	if err != nil {
		return nil, err
	}

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	return &DiskManager{
		file:          file,
		numberOfPages: uint16(info.Size() / PageSize),
	}, nil
}
