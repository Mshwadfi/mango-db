package main

import (
	"fmt"
	"mydb/src/storageLayer"
)

func main() {
	page := storageLayer.NewPage()

	// slot1, _ := page.Insert([]byte("salam"))
	// record1, _ := page.Get(slot1)

	for i := 0; i < 5000; i++ {
		_, err := page.Insert([]byte("salam"))
		if err != nil {
			fmt.Printf("insert failed at i=%d: %v\n", i, err)
			break
		}
	}
}
