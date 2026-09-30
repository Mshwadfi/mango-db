package tablemanager

import "mydb/src/storageLayer"

// this is the scanner used to scan pages of a table and return the record ids of all the records in that table
// the main idea of using this is to avoid loading all records of a table in memory,
// instead we load a page at a time and return the record ids of all the records in that page,
// then move to the next page and repeat the process until we reach the end of the table

// scanner holds at most one page at a time (one pinned page at buffer pool at a time)

type Scanner struct {
	th             *TableHeap
	currentPageId  uint16
	currentPage    *storageLayer.Page // currently pinne dpage
	nextSlotNumber uint16             // next slot to scan in the page
	done           bool               // after scanning all the table this will be true
}

func (th *TableHeap) NewScanner() *Scanner {
	return &Scanner{
		th:             th,
		currentPageId:  th.firstPageId,
		currentPage:    nil,
		nextSlotNumber: 0,
		done:           false,
	}
}

// next returns the live recordIds in the next page
// if there is no next page or no more live records in the current page, it will return nil and done = true

func (s *Scanner) Next() (storageLayer.RecordID, []byte, bool, error) {
	if s.done {
		return storageLayer.RecordID{}, nil, false, nil
	}

	for {
		// if we do not have a current page, we need to fetch it from the buffer manager
		if s.currentPage == nil {
			page, err := s.th.bm.FetchPage(s.currentPageId)
			if err != nil {
				return storageLayer.RecordID{}, nil, false, err
			}
			s.currentPage = page
			s.nextSlotNumber = 0
		}

		// scan the current page for a live record (only one record at a time)
		for s.nextSlotNumber < uint16(s.currentPage.NumberOfSlots()) {
			slotNumber := s.nextSlotNumber
			s.nextSlotNumber++

			record, err := s.currentPage.Get(int(slotNumber))
			if err == storageLayer.ErrSlotDeleted {
				continue // skip deleted slots
			} else if err != nil {
				return storageLayer.RecordID{}, nil, false, err
			}

			// we found a live record, return it
			recordId := storageLayer.RecordID{
				PageId:     s.currentPageId,
				SlotNumber: slotNumber,
			}
			return recordId, record, true, nil

		}

		// if no live records in this current page, we need to unpin it and move to the next page
		s.th.bm.UnPinPage(s.currentPageId, false)
		nextPageId := s.currentPage.GetNextPageId()
		s.currentPage = nil
		s.currentPageId = nextPageId

		if nextPageId == storageLayer.NoNextPageId {
			s.done = true
			return storageLayer.RecordID{}, nil, false, nil
		}

	}
}

// Close releases any page the scanner is currently holding pinned.
// Safe to call multiple times, and safe to call even if Next() already
// ran the scanner to completion (in which case there's nothing to release).
func (s *Scanner) Close() error {
	if s.currentPage != nil {
		s.th.bm.UnPinPage(s.currentPageId, false)
		s.currentPage = nil
	}
	s.done = true
	return nil
}
