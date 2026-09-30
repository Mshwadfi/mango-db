package tablemanager

import "mydb/src/storageLayer"

// this is the parent responsible for collecting pages for the same togethr, and manading them

type TableHeap struct {
	bm          *storageLayer.BufferManager
	firstPageId uint16 // you start traversing from here and use page.next for next page, until you reach lastPageId
	lastPageId  uint16 // you try to insert in this first if it has available space, if not then you allocate a new page and update this to point to the new page
}

func NewTableHeap(bm *storageLayer.BufferManager) (*TableHeap, error) {
	pageId, _, err := bm.NewPage()
	if err != nil {
		return nil, err
	}
	bm.UnPinPage(pageId, false)

	return &TableHeap{
		bm:          bm,
		firstPageId: pageId,
		lastPageId:  pageId,
	}, nil
}

func (th *TableHeap) Insert(record []byte) (storageLayer.RecordID, error) {
	page, err := th.bm.FetchPage(th.lastPageId)
	if err != nil {
		return storageLayer.RecordID{}, err
	}

	slotNumber, err := page.Insert(record)

	if err == storageLayer.ErrNotEnoughSpace {
		newPageId, newPage, newErr := th.bm.NewPage()
		if newErr != nil {
			// if no free space, the compaction runs and this changes the page structure, so we unpin and mark it dirty

			th.bm.UnPinPage(th.lastPageId, true)
			return storageLayer.RecordID{}, newErr
		}

		page.SetNextPageId(newPageId)
		th.bm.UnPinPage(th.lastPageId, true) // old page's header changed

		th.lastPageId = newPageId
		page = newPage

		slotNumber, err = page.Insert(record)
	}

	if err != nil {
		th.bm.UnPinPage(th.lastPageId, false)
		return storageLayer.RecordID{}, err
	}

	th.bm.UnPinPage(th.lastPageId, true)

	return storageLayer.RecordID{
		PageId:     th.lastPageId,
		SlotNumber: uint16(slotNumber),
	}, nil
}

func (th *TableHeap) Get(recordId storageLayer.RecordID) ([]byte, error) {
	page, err := th.bm.FetchPage(recordId.PageId)
	if err != nil {
		return nil, err
	}

	record, err := page.Get(int(recordId.SlotNumber))
	if err != nil {
		th.bm.UnPinPage(recordId.PageId, false)
		return nil, err
	}
	th.bm.UnPinPage(recordId.PageId, false)
	return record, nil
}

func (th *TableHeap) Delete(recordId storageLayer.RecordID) error {
	page, err := th.bm.FetchPage(recordId.PageId)
	if err != nil {
		return err
	}

	err = page.Delete(int(recordId.SlotNumber))
	if err != nil {
		th.bm.UnPinPage(recordId.PageId, false)
		return err
	}

	th.bm.UnPinPage(recordId.PageId, true)
	return nil
}

func (th *TableHeap) Update(recordId storageLayer.RecordID, record []byte) (storageLayer.RecordID, bool, error) {
	page, err := th.bm.FetchPage(recordId.PageId)
	if err != nil {
		return storageLayer.RecordID{}, false, err
	}

	err = page.Update(int(recordId.SlotNumber), record)

	if err == storageLayer.ErrNotEnoughSpace {
		// page.Update runs compact() internally before failing, so the
		// page's bytes changed even though the opration failed.
		th.bm.UnPinPage(recordId.PageId, true)

		newRecordId, insertErr := th.Insert(record)
		if insertErr != nil {
			return storageLayer.RecordID{}, false, insertErr
		}

		if delErr := th.Delete(recordId); delErr != nil {
			// new copy is live at newRecordId; old slot failed to be deleted
			// may be ater we solve this prolem but for now just accept a dead row living in the table,
			//  and return the new record id to the user
			return newRecordId, true, delErr
		}

		return newRecordId, true, nil
	} else if err != nil {
		th.bm.UnPinPage(recordId.PageId, false)
		return storageLayer.RecordID{}, false, err
	}

	th.bm.UnPinPage(recordId.PageId, true)
	return recordId, false, nil
}

// scan functions: iterate throgh all the pages of that table and return []storageLayer.RecordID of all the records  that table
// this will resolve the select * query, the filtering responsibilty is out of this layer
