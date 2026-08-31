package main

import (
	"fmt"
	"log"
	"mydb/src/storageLayer"
)

func main() {

	dm, err := storageLayer.OpenDiskManager("./data/mydb.db")
	if err != nil {
		log.Fatal(err)
	}

	defer dm.Close()

	pageId, err := dm.AllocatePage()
	if err != nil {
		log.Fatal(err)
	}

	page := storageLayer.NewPage()

	slot1, err := page.Insert([]byte("salam"))

	if err != nil {
		log.Fatal(err)
	}

	slot2, err := page.Insert([]byte("ya 3alam"))

	if err != nil {
		log.Fatal(err)
	}

	err = dm.WritePage(pageId, page)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("page:", pageId)
	fmt.Println("slot1:", slot1)
	fmt.Println("slot2:", slot2)
}
