package storageLayer

import (
	"encoding/binary"
	"errors"
)

const (
	PageSize      = 4096
	HeaderSize    = 6
	SlotEntrySize = 5
)

var (
	ErrNotEnoughSpace = errors.New("not enough space in page")
	ErrSlotNotFound   = errors.New("slot not found")
	ErrSlotDeleted    = errors.New("slot is deleted")
)

type PageHeader struct {
	NumberOfSlots  uint16
	FreeSpaceStart uint16
	FreeSpaceEnd   uint16
}

type SlotEntry struct {
	Offset uint16
	Length uint16
	Alive  bool
}

// this will be used by other system component to interact with this layer
//  (other components do not know about offset, slot number)

type RecordID struct {
	PageID uint16
	Slot   uint16
}

type Page struct {
	Data []byte
}

func NewPage() *Page {
	p := &Page{
		Data: make([]byte, PageSize),
	}
	p.setNumberOfSlots(0)
	p.setFreeSpaceStart(HeaderSize)
	p.setFreeSpaceEnd(PageSize)
	return p
}

// --------------------- header methods (private) -----------------------//
func (p *Page) numberOfSlots() int {
	return int(binary.LittleEndian.Uint16(p.Data[0:2]))
}

func (p *Page) setNumberOfSlots(n int) {
	binary.LittleEndian.PutUint16(p.Data[0:2], uint16(n))
}

func (p *Page) freeSpaceStart() int {
	return int(binary.LittleEndian.Uint16(p.Data[2:4]))
}

func (p *Page) setFreeSpaceStart(v int) {
	binary.LittleEndian.PutUint16(p.Data[2:4], uint16(v))
}

func (p *Page) freeSpaceEnd() int {
	return int(binary.LittleEndian.Uint16(p.Data[4:6]))
}

func (p *Page) setFreeSpaceEnd(v int) {
	binary.LittleEndian.PutUint16(p.Data[4:6], uint16(v))
}

// -------------------- slots methods (private) -----------------------//
func slotOffset(slotNumber int) int {
	return HeaderSize + (slotNumber * SlotEntrySize)
}

func (p *Page) getSlot(slotNumber int) (offset int, length int, alive bool) {
	base := slotOffset(slotNumber)
	offset = int(binary.LittleEndian.Uint16(p.Data[base : base+2]))
	length = int(binary.LittleEndian.Uint16(p.Data[base+2 : base+4]))
	alive = p.Data[base+4] == 1

	return
}

func (p *Page) setSlot(slotNumber int, offset int, length int, alive bool) {
	base := slotOffset(slotNumber)
	binary.LittleEndian.PutUint16(p.Data[base:base+2], uint16(offset))
	binary.LittleEndian.PutUint16(p.Data[base+2:base+4], uint16(length))
	if alive {
		p.Data[base+4] = 1
	} else {
		p.Data[base+4] = 0
	}
}

// --------------------- page main methods (public) ------------------------//
func (p *Page) Insert(record []byte) (int, error) {
	recordLength := len(record)
	slotCount := p.numberOfSlots()

	availableSpace := p.freeSpaceEnd() - p.freeSpaceStart()
	neededSpace := SlotEntrySize + recordLength

	if neededSpace > availableSpace {
		p.compact()
		newAvailableSpace := p.freeSpaceEnd() - p.freeSpaceStart()
		if neededSpace > newAvailableSpace {

			return 0, ErrNotEnoughSpace
		}
	}

	newFreeSpaceEnd := p.freeSpaceEnd() - recordLength
	copy(p.Data[newFreeSpaceEnd:p.freeSpaceEnd()], record)

	slotNumber := slotCount
	p.setSlot(slotNumber, newFreeSpaceEnd, recordLength, true)
	p.setNumberOfSlots(slotCount + 1)
	p.setFreeSpaceStart(p.freeSpaceStart() + SlotEntrySize)
	p.setFreeSpaceEnd(newFreeSpaceEnd)

	return slotNumber, nil

}

func (p *Page) Get(slotNumber int) ([]byte, error) {
	if slotNumber < 0 || slotNumber >= p.numberOfSlots() {
		return nil, ErrSlotNotFound
	}

	offset, length, alive := p.getSlot(slotNumber)
	if !alive {
		return nil, ErrSlotDeleted
	}
	record := make([]byte, length)
	copy(record, p.Data[offset:offset+length])
	return record, nil
}

func (p *Page) Delete(slotNumber int) error {
	if slotNumber < 0 || slotNumber >= p.numberOfSlots() {
		return ErrSlotNotFound
	}
	offset, length, alive := p.getSlot(slotNumber)
	if !alive {
		return ErrSlotDeleted
	}
	p.setSlot(slotNumber, offset, length, false)
	return nil
}

func (p *Page) Update(slotNumber int, record []byte) error {
	if slotNumber < 0 || slotNumber >= p.numberOfSlots() {
		return ErrSlotNotFound
	}

	_, _, alive := p.getSlot(slotNumber)
	if !alive {
		return ErrSlotDeleted
	}

	neededSpace := len(record)
	availableSpace := p.freeSpaceEnd() - p.freeSpaceStart()

	if neededSpace > availableSpace {
		p.compact()
		newAvailableSpace := p.freeSpaceEnd() - p.freeSpaceStart()
		if neededSpace > newAvailableSpace {

			return ErrNotEnoughSpace
		}
	}

	newOffset := p.freeSpaceEnd() - len(record)
	copy(p.Data[newOffset:p.freeSpaceEnd()], record)

	p.setSlot(slotNumber, newOffset, len(record), true)
	p.setFreeSpaceEnd(newOffset)

	return nil
}

func (p *Page) compact() {
	oldData := make([]byte, PageSize)
	copy(oldData, p.Data)

	writePos := PageSize

	for slot := 0; slot < p.numberOfSlots(); slot++ {

		offset, length, alive := p.getSlot(slot)
		if !alive {
			continue
		}
		writePos -= length
		copy(p.Data[writePos:writePos+length], oldData[offset:offset+length])
		p.setSlot(slot, writePos, length, true)
	}
	p.setFreeSpaceEnd(writePos)

}
