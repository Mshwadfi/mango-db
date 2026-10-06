package main

import (
	"fmt"
	"log"
	"mydb/src/storageLayer"
	tablemanager "mydb/src/tableManager"
	"mydb/src/tuple"
)

func main() {

	// setup — once per table
	schema, _ := tuple.NewSchema([]tuple.Column{
		{Name: "id", Type: tuple.TypeInt},
		{Name: "name", Type: tuple.TypeVarchar},
		{Name: "age", Type: tuple.TypeInt},
	})

	serializer, _ := tuple.NewSerializer(schema)

	dm, _ := storageLayer.OpenDiskManager("data.db")
	bm, _ := storageLayer.NewBufferManager(10, dm)
	heap, _ := tablemanager.NewTableHeap(bm)

	// insert
	recordIds := make([]storageLayer.RecordID, 100)
	for i := 0; i < 100; i++ {
		row := tuple.NewTuple([]tuple.Value{
			tuple.NewIntValue(int32(i + 1)),
			tuple.NewVarcharValue(fmt.Sprintf("Name%d", i+1)),
			tuple.NewIntValue(int32(20 + i)),
		})

		bytes, err := serializer.Serialize(row)
		if err != nil {
			log.Fatal(err)
		}
		recordId, err := heap.Insert(bytes)
		recordIds[i] = recordId
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Inserted row with record ID: %d\n", recordId)
	}

	// retrieve
	for i := 0; i < 100; i++ {
		raw, err := heap.Get(recordIds[i])
		if err != nil {
			log.Fatal(err)
		}
		rowBack, err := serializer.Deserialize(raw)
		if err != nil {
			log.Fatal(err)
		}

		id, _ := rowBack.Value(0).AsInt()
		name, _ := rowBack.Value(1).AsVarchar()
		age, _ := rowBack.Value(2).AsInt()
		fmt.Printf("Retrieved row: id == %d, name == %q, age == %d\n", id, name, age)
	}
}
