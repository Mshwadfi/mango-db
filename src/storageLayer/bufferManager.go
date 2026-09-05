package storageLayer

import (
	"container/list"
	"errors"
	"sync"
)

type frame struct {
	pageId  uint16
	page    *Page
	isDirty bool
}

type BufferManager struct {
	dm        *DiskManager
	mu        sync.Mutex
	capacity  uint16
	pageTable map[uint16]*list.Element
	lruList   list.List
}

func NewBufferManager(capacity uint16, dm *DiskManager) (*BufferManager, error) {
	if capacity == 0 {
		return nil, errors.New("buffer manager capacity must be greater than zero")
	}

	return &BufferManager{
		dm:        dm,
		capacity:  capacity,
		pageTable: make(map[uint16]*list.Element),
	}, nil
}
func (bm *BufferManager) FetchPage(pageId uint16) (*Page, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// if page exist in cach
	if elem, ok := bm.pageTable[pageId]; ok {
		bm.lruList.MoveToFront(elem)

		return elem.Value.(*frame).page, nil
	}

	// if page is not cached
	if bm.lruList.Len() >= int(bm.capacity) {
		// evict the LRU page
		if err := bm.evictPage(); err != nil {
			return nil, err
		}

	}

	// chach page
	page, err := bm.cachPage(pageId)
	if err != nil {
		return nil, err
	}

	return page, nil

}

func (bm *BufferManager) cachPage(pageId uint16) (*Page, error) {

	page, err := bm.dm.ReadPage(pageId)
	if err != nil {
		return nil, err
	}

	fr := &frame{
		pageId:  pageId,
		page:    page,
		isDirty: false,
	}

	elem := bm.lruList.PushFront(fr)
	bm.pageTable[pageId] = elem

	return page, nil
}

func (bm *BufferManager) evictPage() error {
	elem := bm.lruList.Back()

	if elem == nil {
		return nil
	}

	// check if page is dirty
	fr := elem.Value.(*frame)
	if fr.isDirty {
		if err := bm.dm.WritePage(fr.pageId, fr.page); err != nil {
			return err
		}
	}

	bm.lruList.Remove(elem)
	delete(bm.pageTable, fr.pageId)
	return nil
}

func (bm *BufferManager) MarkDirty(pageId uint16) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if elem, ok := bm.pageTable[pageId]; ok {
		elem.Value.(*frame).isDirty = true
	}

}

func (bm *BufferManager) FlushAll() error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	for elem := bm.lruList.Front(); elem != nil; elem = elem.Next() {
		fr := elem.Value.(*frame)
		if fr.isDirty {
			if err := bm.dm.WritePage(fr.pageId, fr.page); err != nil {
				return err
			}
			fr.isDirty = false
		}
	}

	return nil
}
