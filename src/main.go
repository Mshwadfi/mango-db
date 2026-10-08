package main

import (
	"errors"
	"fmt"
	"log"

	"mydb/src/catalog"
	"mydb/src/storageLayer"
	"mydb/src/tuple"
)

func main() {
	dm, err := storageLayer.OpenDiskManager("data.db")
	if err != nil {
		log.Fatal(err)
	}
	bm, err := storageLayer.NewBufferManager(10, dm)
	if err != nil {
		log.Fatal(err)
	}

	cat, err := catalog.OpenCatalog(bm, dm)
	if err != nil {
		log.Fatal(err)
	}

	userSchema, err := tuple.NewSchema([]tuple.Column{
		{Name: "id", Type: tuple.TypeInt},
		{Name: "name", Type: tuple.TypeVarchar},
		{Name: "age", Type: tuple.TypeInt},
	})
	if err != nil {
		log.Fatal(err)
	}

	// Try to open the table first. If it doesn't exist yet, create it.
	// This is the key difference from before: a second run should hit
	// GetTable successfully, not fall through to CreateTable again.
	heap, foundSchema, err := cat.GetTable("users")
	if errors.Is(err, catalog.ErrTableNotFound) {
		fmt.Println("users table not found — creating it")
		heap, err = cat.CreateTable("users", userSchema)
		if err != nil {
			log.Fatal(err)
		}
		foundSchema = userSchema
	} else if err != nil {
		log.Fatal(err)
	} else {
		fmt.Println("users table found in catalog — reusing it")
	}

	fmt.Printf("users table's firstPageId: %d\n", heap.FirstPageId())

	serializer, err := tuple.NewSerializer(foundSchema)
	if err != nil {
		log.Fatal(err)
	}

	// Insert a few rows — call this program multiple times and watch
	// the row count grow, proving data survives across runs.
	for i := 0; i < 3; i++ {
		row := tuple.NewTuple([]tuple.Value{
			tuple.NewIntValue(int32(i + 1)),
			tuple.NewVarcharValue(fmt.Sprintf("Name%d", i+1)),
			tuple.NewIntValue(int32(20 + i)),
		})
		data, err := serializer.Serialize(row)
		if err != nil {
			log.Fatal(err)
		}
		recordId, err := heap.Insert(data)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Inserted row with record ID: %v\n", recordId)
	}

	// Scan everything currently in the table — including rows from
	// previous runs, if the catalog is working correctly.
	scanner := heap.NewScanner()
	defer scanner.Close()

	count := 0
	for {
		_, data, ok, err := scanner.Next()
		if err != nil {
			log.Fatal(err)
		}
		if !ok {
			break
		}
		row, err := serializer.Deserialize(data)
		if err != nil {
			log.Fatal(err)
		}
		id, _ := row.Value(0).AsInt()
		name, _ := row.Value(1).AsVarchar()
		age, _ := row.Value(2).AsInt()
		fmt.Printf("Row: id=%d, name=%q, age=%d\n", id, name, age)
		count++
	}
	fmt.Printf("Total rows in table: %d\n", count)
}
