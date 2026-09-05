package storageLayer

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func newTestDiskManager(t *testing.T) *DiskManager {
	t.Helper()

	path := t.TempDir() + "/test.db"

	dm, err := OpenDiskManager(path)
	if err != nil {
		t.Fatalf("failed to open disk manager: %v", err)
	}

	t.Cleanup(func() {
		if err := dm.Close(); err != nil {
			t.Errorf("failed to close disk manager: %v", err)
		}
	})

	return dm
}

func allocatePages(t *testing.T, dm *DiskManager, count int) []uint16 {
	t.Helper()

	pageIDs := make([]uint16, count)

	for i := 0; i < count; i++ {
		pageID, err := dm.AllocatePage()
		if err != nil {
			t.Fatalf("failed to allocate page %d: %v", i, err)
		}

		pageIDs[i] = pageID
	}

	return pageIDs
}

func getFrame(t *testing.T, bm *BufferManager, pageID uint16) *frame {
	t.Helper()

	elem, ok := bm.pageTable[pageID]
	if !ok {
		t.Fatalf("page %d not found in buffer pool", pageID)
	}

	return elem.Value.(*frame)
}

func TestNewBufferManager(t *testing.T) {
	dm := newTestDiskManager(t)

	bm, err := NewBufferManager(3, dm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bm.capacity != 3 {
		t.Fatalf("expected capacity 3, got %d", bm.capacity)
	}

	if len(bm.pageTable) != 0 {
		t.Fatalf(
			"expected empty page table, got %d entries",
			len(bm.pageTable),
		)
	}

	if bm.lruList.Len() != 0 {
		t.Fatalf(
			"expected empty LRU list, got %d",
			bm.lruList.Len(),
		)
	}
}

func TestNewBufferManager_ZeroCapacity(t *testing.T) {
	dm := newTestDiskManager(t)

	_, err := NewBufferManager(0, dm)

	if !errors.Is(err, ErrBufferMinCapacity) {
		t.Fatalf(
			"expected ErrBufferMinCapacity, got %v",
			err,
		)
	}
}

func TestBufferManager_FetchPage(t *testing.T) {
	dm := newTestDiskManager(t)

	pageID, err := dm.AllocatePage()
	if err != nil {
		t.Fatal(err)
	}

	bm, err := NewBufferManager(2, dm)
	if err != nil {
		t.Fatal(err)
	}

	page, err := bm.FetchPage(pageID)
	if err != nil {
		t.Fatalf("FetchPage failed: %v", err)
	}

	if page == nil {
		t.Fatal("expected page, got nil")
	}

	if bm.lruList.Len() != 1 {
		t.Fatalf(
			"expected 1 frame, got %d",
			bm.lruList.Len(),
		)
	}

	fr := getFrame(t, bm, pageID)

	if fr.pageId != pageID {
		t.Fatalf(
			"expected page ID %d, got %d",
			pageID,
			fr.pageId,
		)
	}

	if fr.pinCount != 1 {
		t.Fatalf(
			"expected pin count 1, got %d",
			fr.pinCount,
		)
	}

	if fr.isDirty {
		t.Fatal("newly fetched page should not be dirty")
	}
}

func TestBufferManager_FetchPage_CacheHit(t *testing.T) {
	dm := newTestDiskManager(t)

	pageID, err := dm.AllocatePage()
	if err != nil {
		t.Fatal(err)
	}

	bm, err := NewBufferManager(2, dm)
	if err != nil {
		t.Fatal(err)
	}

	page1, err := bm.FetchPage(pageID)
	if err != nil {
		t.Fatal(err)
	}

	page2, err := bm.FetchPage(pageID)
	if err != nil {
		t.Fatal(err)
	}

	if page1 != page2 {
		t.Fatal("cache hit should return the same page pointer")
	}

	if bm.lruList.Len() != 1 {
		t.Fatalf(
			"expected 1 frame, got %d",
			bm.lruList.Len(),
		)
	}

	fr := getFrame(t, bm, pageID)

	if fr.pinCount != 2 {
		t.Fatalf(
			"expected pin count 2, got %d",
			fr.pinCount,
		)
	}
}

func TestBufferManager_UnpinPage(t *testing.T) {
	dm := newTestDiskManager(t)

	pageID, err := dm.AllocatePage()
	if err != nil {
		t.Fatal(err)
	}

	bm, err := NewBufferManager(2, dm)
	if err != nil {
		t.Fatal(err)
	}

	_, err = bm.FetchPage(pageID)
	if err != nil {
		t.Fatal(err)
	}

	if err := bm.UnPinPage(pageID, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fr := getFrame(t, bm, pageID)

	if fr.pinCount != 0 {
		t.Fatalf(
			"expected pin count 0, got %d",
			fr.pinCount,
		)
	}
}

func TestBufferManager_PinCount(t *testing.T) {
	dm := newTestDiskManager(t)

	pageID, err := dm.AllocatePage()
	if err != nil {
		t.Fatal(err)
	}

	bm, err := NewBufferManager(2, dm)
	if err != nil {
		t.Fatal(err)
	}

	for expected := uint16(1); expected <= 5; expected++ {
		_, err := bm.FetchPage(pageID)
		if err != nil {
			t.Fatal(err)
		}

		fr := getFrame(t, bm, pageID)

		if fr.pinCount != expected {
			t.Fatalf(
				"expected pin count %d, got %d",
				expected,
				fr.pinCount,
			)
		}
	}

	for expected := 4; expected >= 0; expected-- {
		err := bm.UnPinPage(pageID, false)
		if err != nil {
			t.Fatal(err)
		}

		fr := getFrame(t, bm, pageID)

		if fr.pinCount != uint16(expected) {
			t.Fatalf(
				"expected pin count %d, got %d",
				expected,
				fr.pinCount,
			)
		}
	}
}

func TestBufferManager_UnpinPage_AlreadyZero(t *testing.T) {
	dm := newTestDiskManager(t)

	pageID, err := dm.AllocatePage()
	if err != nil {
		t.Fatal(err)
	}

	bm, err := NewBufferManager(2, dm)
	if err != nil {
		t.Fatal(err)
	}

	_, err = bm.FetchPage(pageID)
	if err != nil {
		t.Fatal(err)
	}

	if err := bm.UnPinPage(pageID, false); err != nil {
		t.Fatal(err)
	}

	err = bm.UnPinPage(pageID, false)

	if !errors.Is(err, ErrPinCountIsAlreadyZero) {
		t.Fatalf(
			"expected ErrPinCountIsAlreadyZero, got %v",
			err,
		)
	}
}

func TestBufferManager_UnpinPage_NotFound(t *testing.T) {
	dm := newTestDiskManager(t)

	bm, err := NewBufferManager(2, dm)
	if err != nil {
		t.Fatal(err)
	}

	err = bm.UnPinPage(999, false)

	if !errors.Is(err, ErrPageNotFoundInBufferPool) {
		t.Fatalf(
			"expected ErrPageNotFoundInBufferPool, got %v",
			err,
		)
	}
}

func TestBufferManager_LRUOrder(t *testing.T) {
	dm := newTestDiskManager(t)

	pageIDs := allocatePages(t, dm, 3)

	bm, err := NewBufferManager(3, dm)
	if err != nil {
		t.Fatal(err)
	}

	// Fetch 0
	_, err = bm.FetchPage(pageIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	bm.UnPinPage(pageIDs[0], false)

	// Fetch 1
	_, err = bm.FetchPage(pageIDs[1])
	if err != nil {
		t.Fatal(err)
	}
	bm.UnPinPage(pageIDs[1], false)

	// Fetch 2
	_, err = bm.FetchPage(pageIDs[2])
	if err != nil {
		t.Fatal(err)
	}
	bm.UnPinPage(pageIDs[2], false)

	// Make page 0 most recently used.
	_, err = bm.FetchPage(pageIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	bm.UnPinPage(pageIDs[0], false)

	expected := []uint16{
		pageIDs[0],
		pageIDs[2],
		pageIDs[1],
	}

	i := 0

	for elem := bm.lruList.Front(); elem != nil; elem = elem.Next() {
		fr := elem.Value.(*frame)

		if fr.pageId != expected[i] {
			t.Fatalf(
				"LRU position %d: expected page %d, got %d",
				i,
				expected[i],
				fr.pageId,
			)
		}

		i++
	}
}

func TestBufferManager_EvictLRUPage(t *testing.T) {
	dm := newTestDiskManager(t)

	pageIDs := allocatePages(t, dm, 3)

	bm, err := NewBufferManager(2, dm)
	if err != nil {
		t.Fatal(err)
	}

	_, err = bm.FetchPage(pageIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	bm.UnPinPage(pageIDs[0], false)

	_, err = bm.FetchPage(pageIDs[1])
	if err != nil {
		t.Fatal(err)
	}
	bm.UnPinPage(pageIDs[1], false)

	// Page 0 is now LRU.
	_, err = bm.FetchPage(pageIDs[2])
	if err != nil {
		t.Fatal(err)
	}
	bm.UnPinPage(pageIDs[2], false)

	if _, ok := bm.pageTable[pageIDs[0]]; ok {
		t.Fatal("expected LRU page to be evicted")
	}

	if _, ok := bm.pageTable[pageIDs[1]]; !ok {
		t.Fatal("expected page 1 to remain")
	}

	if _, ok := bm.pageTable[pageIDs[2]]; !ok {
		t.Fatal("expected page 2 to be cached")
	}
}

func TestBufferManager_AllPagesPinned(t *testing.T) {
	dm := newTestDiskManager(t)

	pageIDs := allocatePages(t, dm, 3)

	bm, err := NewBufferManager(2, dm)
	if err != nil {
		t.Fatal(err)
	}

	// Both pages remain pinned.
	_, err = bm.FetchPage(pageIDs[0])
	if err != nil {
		t.Fatal(err)
	}

	_, err = bm.FetchPage(pageIDs[1])
	if err != nil {
		t.Fatal(err)
	}

	_, err = bm.FetchPage(pageIDs[2])

	if !errors.Is(err, ErrNoUnpinnedFrames) {
		t.Fatalf(
			"expected ErrNoUnpinnedFrames, got %v",
			err,
		)
	}

	if bm.lruList.Len() != 2 {
		t.Fatalf(
			"expected 2 frames, got %d",
			bm.lruList.Len(),
		)
	}

	if _, ok := bm.pageTable[pageIDs[0]]; !ok {
		t.Fatal("pinned page 0 was incorrectly evicted")
	}

	if _, ok := bm.pageTable[pageIDs[1]]; !ok {
		t.Fatal("pinned page 1 was incorrectly evicted")
	}

	// Cleanup.
	bm.UnPinPage(pageIDs[0], false)
	bm.UnPinPage(pageIDs[1], false)
}

func TestBufferManager_EvictionSkipsPinnedPages(t *testing.T) {
	dm := newTestDiskManager(t)

	pageIDs := allocatePages(t, dm, 4)

	bm, err := NewBufferManager(3, dm)
	if err != nil {
		t.Fatal(err)
	}

	// Page 0 -> unpinned
	_, err = bm.FetchPage(pageIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	bm.UnPinPage(pageIDs[0], false)

	// Page 1 -> unpinned
	_, err = bm.FetchPage(pageIDs[1])
	if err != nil {
		t.Fatal(err)
	}
	bm.UnPinPage(pageIDs[1], false)

	// Page 2 -> pinned
	_, err = bm.FetchPage(pageIDs[2])
	if err != nil {
		t.Fatal(err)
	}

	// Make page 0 more recently used.
	_, err = bm.FetchPage(pageIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	bm.UnPinPage(pageIDs[0], false)

	/*
		LRU:

		front
		  0  unpinned
		  2  pinned
		  1  unpinned
		back

		Page 1 should be evicted.
	*/

	_, err = bm.FetchPage(pageIDs[3])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := bm.pageTable[pageIDs[1]]; ok {
		t.Fatal("expected page 1 to be evicted")
	}

	if _, ok := bm.pageTable[pageIDs[2]]; !ok {
		t.Fatal("pinned page 2 was incorrectly evicted")
	}

	// Cleanup.
	bm.UnPinPage(pageIDs[2], false)
	bm.UnPinPage(pageIDs[3], false)
}

func TestBufferManager_DirtyPageWrittenOnEviction(t *testing.T) {
	dm := newTestDiskManager(t)

	pageIDs := allocatePages(t, dm, 2)

	bm, err := NewBufferManager(1, dm)
	if err != nil {
		t.Fatal(err)
	}

	page, err := bm.FetchPage(pageIDs[0])
	if err != nil {
		t.Fatal(err)
	}

	page.Data[0] = 42

	if err := bm.UnPinPage(pageIDs[0], true); err != nil {
		t.Fatal(err)
	}

	// Capacity is 1, so this forces page 0 out.
	_, err = bm.FetchPage(pageIDs[1])
	if err != nil {
		t.Fatal(err)
	}

	// Read directly from disk.
	persisted, err := dm.ReadPage(pageIDs[0])
	if err != nil {
		t.Fatal(err)
	}

	if persisted.Data[0] != 42 {
		t.Fatalf(
			"expected persisted byte 42, got %d",
			persisted.Data[0],
		)
	}

	bm.UnPinPage(pageIDs[1], false)
}

func TestBufferManager_FlushAll(t *testing.T) {
	dm := newTestDiskManager(t)

	pageIDs := allocatePages(t, dm, 2)

	bm, err := NewBufferManager(2, dm)
	if err != nil {
		t.Fatal(err)
	}

	page, err := bm.FetchPage(pageIDs[0])
	if err != nil {
		t.Fatal(err)
	}

	page.Data[0] = 99

	if err := bm.UnPinPage(pageIDs[0], true); err != nil {
		t.Fatal(err)
	}

	if err := bm.FlushAll(); err != nil {
		t.Fatalf("FlushAll failed: %v", err)
	}

	fr := getFrame(t, bm, pageIDs[0])

	if fr.isDirty {
		t.Fatal("expected dirty flag to be cleared after FlushAll")
	}

	persisted, err := dm.ReadPage(pageIDs[0])
	if err != nil {
		t.Fatal(err)
	}

	if persisted.Data[0] != 99 {
		t.Fatalf(
			"expected persisted byte 99, got %d",
			persisted.Data[0],
		)
	}
}

func TestBufferManager_FetchAfterEviction(t *testing.T) {
	dm := newTestDiskManager(t)

	pageIDs := allocatePages(t, dm, 2)

	bm, err := NewBufferManager(1, dm)
	if err != nil {
		t.Fatal(err)
	}

	page, err := bm.FetchPage(pageIDs[0])
	if err != nil {
		t.Fatal(err)
	}

	page.Data[0] = 123

	if err := bm.UnPinPage(pageIDs[0], true); err != nil {
		t.Fatal(err)
	}

	// Evict page 0.
	page2, err := bm.FetchPage(pageIDs[1])
	if err != nil {
		t.Fatal(err)
	}

	bm.UnPinPage(pageIDs[1], false)

	// Page 0 is no longer cached.
	if _, ok := bm.pageTable[pageIDs[0]]; ok {
		t.Fatal("expected page 0 to be evicted")
	}

	// Fetch it again from disk.
	pageAgain, err := bm.FetchPage(pageIDs[0])
	if err != nil {
		t.Fatal(err)
	}

	if pageAgain.Data[0] != 123 {
		t.Fatalf(
			"expected value 123 after reload, got %d",
			pageAgain.Data[0],
		)
	}

	bm.UnPinPage(pageIDs[0], false)

	_ = page2
}

// concurrency test

func TestBufferManager_ConcurrentFetchUnpin(t *testing.T) {
	const (
		capacity               = 10
		numPages               = 20
		numGoroutines          = 50
		operationsPerGoroutine = 500
	)

	dm := newTestDiskManager(t)
	pageIDs := allocatePages(t, dm, numPages)

	bm, err := NewBufferManager(capacity, dm)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup

	var noRoomCount atomic.Int64
	var operationCount atomic.Int64

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)

		go func(seed int) {
			defer wg.Done()

			for i := 0; i < operationsPerGoroutine; i++ {
				index := (seed + i) % numPages
				pageID := pageIDs[index]

				_, err := bm.FetchPage(pageID)

				if errors.Is(err, ErrNoUnpinnedFrames) {
					noRoomCount.Add(1)
					continue
				}

				if err != nil {
					t.Errorf(
						"FetchPage(%d) failed: %v",
						pageID,
						err,
					)
					continue
				}

				operationCount.Add(1)

				dirty := i%10 == 0

				if err := bm.UnPinPage(pageID, dirty); err != nil {
					t.Errorf(
						"UnPinPage(%d) failed: %v",
						pageID,
						err,
					)
				}
			}
		}(g)
	}

	wg.Wait()

	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Every successful Fetch should eventually have been unpinned.
	for elem := bm.lruList.Front(); elem != nil; elem = elem.Next() {
		fr := elem.Value.(*frame)

		if fr.pinCount != 0 {
			t.Errorf(
				"page %d leaked pin count %d",
				fr.pageId,
				fr.pinCount,
			)
		}
	}

	if bm.lruList.Len() > int(bm.capacity) {
		t.Fatalf(
			"buffer pool exceeded capacity: %d > %d",
			bm.lruList.Len(),
			bm.capacity,
		)
	}

	if bm.lruList.Len() != len(bm.pageTable) {
		t.Fatalf(
			"LRU/page table mismatch: list=%d table=%d",
			bm.lruList.Len(),
			len(bm.pageTable),
		)
	}

	t.Logf(
		"successful operations: %d, no-room errors: %d",
		operationCount.Load(),
		noRoomCount.Load(),
	)
}

func TestBufferManager_ConcurrentSamePage(t *testing.T) {
	const (
		numGoroutines = 100
		operations    = 100
	)

	dm := newTestDiskManager(t)

	pageID, err := dm.AllocatePage()
	if err != nil {
		t.Fatal(err)
	}

	bm, err := NewBufferManager(2, dm)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for i := 0; i < operations; i++ {
				_, err := bm.FetchPage(pageID)
				if err != nil {
					t.Errorf("FetchPage failed: %v", err)
					continue
				}

				if err := bm.UnPinPage(pageID, false); err != nil {
					t.Errorf("UnPinPage failed: %v", err)
				}
			}
		}()
	}

	wg.Wait()

	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.lruList.Len() != 1 {
		t.Fatalf(
			"expected exactly one frame, got %d",
			bm.lruList.Len(),
		)
	}

	fr := getFrame(t, bm, pageID)

	if fr.pinCount != 0 {
		t.Fatalf(
			"expected pin count 0, got %d",
			fr.pinCount,
		)
	}
}

func TestBufferManager_ConcurrentDifferentPages(t *testing.T) {
	const (
		numPages      = 50
		numGoroutines = 50
	)

	dm := newTestDiskManager(t)
	pageIDs := allocatePages(t, dm, numPages)

	bm, err := NewBufferManager(10, dm)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)

		go func(index int) {
			defer wg.Done()

			pageID := pageIDs[index]

			_, err := bm.FetchPage(pageID)
			if err != nil {
				// ErrNoUnpinnedFrames is possible because
				// every page may initially remain pinned.
				if !errors.Is(err, ErrNoUnpinnedFrames) {
					t.Errorf(
						"FetchPage(%d): %v",
						pageID,
						err,
					)
				}
				return
			}

			if err := bm.UnPinPage(pageID, false); err != nil {
				t.Errorf(
					"UnPinPage(%d): %v",
					pageID,
					err,
				)
			}
		}(g)
	}

	wg.Wait()

	bm.mu.Lock()
	defer bm.mu.Unlock()

	for elem := bm.lruList.Front(); elem != nil; elem = elem.Next() {
		fr := elem.Value.(*frame)

		if fr.pinCount != 0 {
			t.Errorf(
				"page %d leaked pin count %d",
				fr.pageId,
				fr.pinCount,
			)
		}
	}
}

func TestBufferManager_ConcurrentDirtyPages(t *testing.T) {
	const (
		numPages      = 50
		numGoroutines = 50
		operations    = 100
	)

	dm := newTestDiskManager(t)
	pageIDs := allocatePages(t, dm, numPages)

	bm, err := NewBufferManager(10, dm)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)

		go func(seed int) {
			defer wg.Done()

			for i := 0; i < operations; i++ {
				pageID := pageIDs[seed%numPages]

				page, err := bm.FetchPage(pageID)
				if err != nil {
					if errors.Is(err, ErrNoUnpinnedFrames) {
						continue
					}

					t.Errorf(
						"FetchPage(%d): %v",
						pageID,
						err,
					)
					continue
				}

				// Each goroutine owns a different page.
				page.Data[0] = byte(seed)

				if err := bm.UnPinPage(pageID, true); err != nil {
					t.Errorf(
						"UnPinPage(%d): %v",
						pageID,
						err,
					)
				}
			}
		}(g)
	}

	wg.Wait()

	if err := bm.FlushAll(); err != nil {
		t.Fatalf("FlushAll failed: %v", err)
	}

	bm.mu.Lock()
	defer bm.mu.Unlock()

	for elem := bm.lruList.Front(); elem != nil; elem = elem.Next() {
		fr := elem.Value.(*frame)

		if fr.pinCount != 0 {
			t.Errorf(
				"page %d leaked pin count %d",
				fr.pageId,
				fr.pinCount,
			)
		}

		if fr.isDirty {
			t.Errorf(
				"page %d still dirty after FlushAll",
				fr.pageId,
			)
		}
	}
}
