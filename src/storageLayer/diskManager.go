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

var ErrPageOutOfRange = errors.New("page ID out of range")

func OpenDiskManager(path string) (*DiskManager, error) {
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

func (dm *DiskManager) AllocatePage() (uint16, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	pageId := dm.numberOfPages
	offset := int64(pageId) * PageSize // writeAt expects offset as int64

	page := NewPage()
	_, err := dm.file.WriteAt(page.Data, offset)

	if err != nil {
		return 0, err
	}

	dm.numberOfPages++
	return pageId, nil
}

func (dm *DiskManager) ReadPage(pageId uint16) (*Page, error) {
	dm.mu.Lock() // change it later to RWmutex for concurrent read, exclusive write
	defer dm.mu.Unlock()

	if pageId >= dm.numberOfPages {
		return nil, ErrPageOutOfRange
	}
	buffer := make([]byte, PageSize)
	offset := int64(pageId) * PageSize

	_, err := dm.file.ReadAt(buffer, offset)
	if err != nil {
		return nil, err
	}

	return &Page{Data: buffer}, nil
}

func (dm *DiskManager) WritePage(pageId uint16, p *Page) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if pageId >= dm.numberOfPages {
		return ErrPageOutOfRange
	}

	offset := int64(pageId) * PageSize
	_, err := dm.file.WriteAt(p.Data, offset)

	if err != nil {
		return err
	}

	return dm.file.Sync()
}

func (dm *DiskManager) Close() error {
	return dm.file.Close()
}
