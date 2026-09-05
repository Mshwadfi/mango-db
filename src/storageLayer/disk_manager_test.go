package storageLayer

import (
	"errors"
	"os"
	"testing"
)

func TestOpenDiskManager(t *testing.T) {
	path := t.TempDir() + "/test.db"

	dm, err := OpenDiskManager(path)
	if err != nil {
		t.Fatalf("failed to open disk manager: %v", err)
	}
	defer dm.Close()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database file was not created: %v", err)
	}

	_, err = dm.ReadPage(0)
	if !errors.Is(err, ErrPageOutOfRange) {
		t.Fatalf("expected ErrPageOutOfRange, got %v", err)
	}
}

func TestAllocatePage(t *testing.T) {
	path := t.TempDir() + "/test.db"

	dm, err := OpenDiskManager(path)
	if err != nil {
		t.Fatal(err)
	}
	defer dm.Close()

	pageID, err := dm.AllocatePage()
	if err != nil {
		t.Fatalf("failed to allocate page: %v", err)
	}

	if pageID != 0 {
		t.Fatalf("expected page ID 0, got %d", pageID)
	}

	_, err = dm.ReadPage(pageID)
	if err != nil {
		t.Fatalf("failed to read allocated page: %v", err)
	}
}

func TestAllocateMultiplePages(t *testing.T) {
	path := t.TempDir() + "/test.db"

	dm, err := OpenDiskManager(path)
	if err != nil {
		t.Fatal(err)
	}
	defer dm.Close()

	for expectedID := uint16(0); expectedID < 5; expectedID++ {
		pageID, err := dm.AllocatePage()
		if err != nil {
			t.Fatalf("failed to allocate page %d: %v", expectedID, err)
		}

		if pageID != expectedID {
			t.Fatalf(
				"expected page ID %d, got %d",
				expectedID,
				pageID,
			)
		}
	}
}

func TestReadPageOutOfRange(t *testing.T) {
	path := t.TempDir() + "/test.db"

	dm, err := OpenDiskManager(path)
	if err != nil {
		t.Fatal(err)
	}
	defer dm.Close()

	_, err = dm.ReadPage(0)

	if !errors.Is(err, ErrPageOutOfRange) {
		t.Fatalf("expected ErrPageOutOfRange, got %v", err)
	}
}

func TestWritePageOutOfRange(t *testing.T) {
	path := t.TempDir() + "/test.db"

	dm, err := OpenDiskManager(path)
	if err != nil {
		t.Fatal(err)
	}
	defer dm.Close()

	page := NewPage()

	err = dm.WritePage(0, page)

	if !errors.Is(err, ErrPageOutOfRange) {
		t.Fatalf("expected ErrPageOutOfRange, got %v", err)
	}
}

func TestWriteAndReadPage(t *testing.T) {
	path := t.TempDir() + "/test.db"

	dm, err := OpenDiskManager(path)
	if err != nil {
		t.Fatal(err)
	}
	defer dm.Close()

	pageID, err := dm.AllocatePage()
	if err != nil {
		t.Fatal(err)
	}

	page := NewPage()

	_, err = page.Insert([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}

	err = dm.WritePage(pageID, page)
	if err != nil {
		t.Fatalf("failed to write page: %v", err)
	}

	readPage, err := dm.ReadPage(pageID)
	if err != nil {
		t.Fatalf("failed to read page: %v", err)
	}

	record, err := readPage.Get(0)
	if err != nil {
		t.Fatalf("failed to get record: %v", err)
	}

	if string(record) != "hello" {
		t.Fatalf(
			"expected %q, got %q",
			"hello",
			string(record),
		)
	}
}
