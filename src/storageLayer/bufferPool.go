package storageLayer

import (
	"container/list"
	"errors"
	"sync"
)

var ErrNoUnpinnedFrames = errors.New("buffer pool: no unpinned frames available for eviction")

var ErrBufferMinCapacity = errors.New("buffer manager capacity must be greater than zero")

var ErrPageNotFoundInBufferPool = errors.New("unpin called on page not in buffer pool")

var ErrPinCountIsAlreadyZero = errors.New("unpin called on page with pin count already zero")

type frame struct {
	pageId   uint16
	page     *Page
	isDirty  bool
	pinCount uint16
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
		return nil, ErrBufferMinCapacity
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
		fr := elem.Value.(*frame)
		fr.pinCount++
		bm.lruList.MoveToFront(elem)
		return fr.page, nil
	}

	// if page is not cached
	if bm.lruList.Len() >= int(bm.capacity) {
		// evict the LRU page
		if err := bm.evictPage(); err != nil {
			return nil, err
		}

	}

	// chach page
	page, err := bm.cachePage(pageId)
	if err != nil {
		return nil, err
	}

	return page, nil

}

// UnPinPage decrements the pin count for pageId. It must be called exactly
// once for every successful call to FetchPage on that page. Failing to
// call it leaks the frame (it can never be evicted); calling it more times
// than FetchPage was called returns ErrPinCountIsAlreadyZero.

func (bm *BufferManager) UnPinPage(pageId uint16, isDirty bool) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	elem, ok := bm.pageTable[pageId]
	if !ok {
		return ErrPageNotFoundInBufferPool
	}

	fr := elem.Value.(*frame)
	if fr.pinCount == 0 {
		return ErrPinCountIsAlreadyZero
	}

	fr.pinCount--

	if isDirty {
		fr.isDirty = true
	}

	return nil
}
func (bm *BufferManager) cachePage(pageId uint16) (*Page, error) {

	page, err := bm.dm.ReadPage(pageId)
	if err != nil {
		return nil, err
	}

	fr := &frame{
		pageId:   pageId,
		page:     page,
		isDirty:  false,
		pinCount: 1,
	}

	elem := bm.lruList.PushFront(fr)
	bm.pageTable[pageId] = elem

	return page, nil
}

// travers the lru list nodes from back to find page that is not currently used by any transaction (pincount = 0)
// and evict this page, if no page found then return error (all pages are currently being used)

func (bm *BufferManager) evictPage() error {
	for elem := bm.lruList.Back(); elem != nil; elem = elem.Prev() {
		fr := elem.Value.(*frame)

		if fr.pinCount > 0 {
			continue
		}

		if fr.isDirty {
			if err := bm.dm.WritePage(fr.pageId, fr.page); err != nil {
				return err
			}

		}
		bm.lruList.Remove(elem)
		delete(bm.pageTable, fr.pageId)

		return nil
	}
	return ErrNoUnpinnedFrames
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

func (bm *BufferManager) NewPage() (uint16, *Page, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// if buffer pool is full, evict a page
	if bm.lruList.Len() >= int(bm.capacity) {
		if err := bm.evictPage(); err != nil {
			return 0, nil, err
		}
	}

	// create the page on disk first
	pageId, err := bm.dm.AllocatePage()
	if err != nil {
		return 0, nil, err
	}

	// cache the page in buffer pool
	page, err := bm.cachePage(pageId)
	if err != nil {
		return 0, nil, err
	}

	return pageId, page, nil
}
