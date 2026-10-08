package catalog

import (
	"errors"
	"fmt"

	"mydb/src/storageLayer"
	tablemanager "mydb/src/tableManager"
	"mydb/src/tuple"
)

var (
	ErrTableNotFound = errors.New("catalog: table not found")
	ErrTableExists   = errors.New("catalog: table already exists")
)

// catalogSchema describes the catalog's own storage format. It is fixed
// and known at compile time — not something a caller defines, unlike a
// user table's schema.
func catalogSchema() *tuple.Schema {
	schema, err := tuple.NewSchema([]tuple.Column{
		{Name: "table_name", Type: tuple.TypeVarchar},
		{Name: "first_page_id", Type: tuple.TypeBigInt},
		{Name: "schema_bytes", Type: tuple.TypeVarchar},
	})
	if err != nil {
		// Only reachable if the hardcoded schema above is malformed —
		// a programming error, not a runtime condition. Panic is
		// appropriate here, same as an unrecoverable invariant violation.
		panic(fmt.Sprintf("catalog: invalid internal schema: %v", err))
	}
	return schema
}

// Catalog is the database's table registry — it durably tracks every user
// table's name, starting page, and schema, so tables survive a restart.
// It is itself stored as an ordinary TableHeap, rooted at the fixed,
// well-known page 0.
type Catalog struct {
	bm         *storageLayer.BufferManager
	heap       *tablemanager.TableHeap
	schema     *tuple.Schema // the catalog's own fixed schema
	serializer *tuple.Serializer
}

const catalogRootPageId = 0

// OpenCatalog opens the database's catalog, initializing it if this is a
// brand-new database file (no pages exist yet) or resuming the existing
// one otherwise.
func OpenCatalog(bm *storageLayer.BufferManager, dm *storageLayer.DiskManager) (*Catalog, error) {
	schema := catalogSchema()
	serializer, err := tuple.NewSerializer(schema)
	if err != nil {
		return nil, err
	}

	var heap *tablemanager.TableHeap

	if dm.GetNumberOfPages() == 0 {
		// Fresh file: nothing exists yet. NewTableHeap allocates the
		// very first page via bm.NewPage(), which — since nothing has
		// been allocated before — will hand back page 0, making the
		// catalog's root page 0 by construction, not by special-casing.
		heap, err = tablemanager.NewTableHeap(bm)
		if err != nil {
			return nil, err
		}
	} else {
		// Existing file: page 0 already holds the catalog. Resume it —
		// never allocate here, that would stomp existing data.
		heap, err = tablemanager.OpenTableHeap(bm, catalogRootPageId)
		if err != nil {
			return nil, err
		}
	}

	return &Catalog{
		bm:         bm,
		heap:       heap,
		schema:     schema,
		serializer: serializer,
	}, nil
}

// CreateTable registers a new table with the given name and schema,
// allocates its storage, and returns a ready-to-use TableHeap for it.
func (c *Catalog) CreateTable(name string, schema *tuple.Schema) (*tablemanager.TableHeap, error) {
	if _, _, err := c.lookup(name); err == nil {
		return nil, ErrTableExists
	} else if !errors.Is(err, ErrTableNotFound) {
		return nil, err // a real scan error, not "just not found"
	}

	tableHeap, err := tablemanager.NewTableHeap(c.bm)
	if err != nil {
		return nil, err
	}

	if err := c.insertEntry(name, tableHeap, schema); err != nil {
		return nil, err
	}

	return tableHeap, nil
}

// GetTable looks up an existing table by name, returning a TableHeap
// reopened at its stored first page, along with its schema.
func (c *Catalog) GetTable(name string) (*tablemanager.TableHeap, *tuple.Schema, error) {
	firstPageId, userSchema, err := c.lookup(name)
	if err != nil {
		return nil, nil, err
	}

	tableHeap, err := tablemanager.OpenTableHeap(c.bm, firstPageId)
	if err != nil {
		return nil, nil, err
	}

	return tableHeap, userSchema, nil
}

// lookup scans the catalog's own heap for a row matching name.
func (c *Catalog) lookup(name string) (firstPageId uint16, userSchema *tuple.Schema, err error) {
	scanner := c.heap.NewScanner()
	defer scanner.Close()

	for {
		_, data, ok, scanErr := scanner.Next()
		if scanErr != nil {
			return 0, nil, scanErr
		}
		if !ok {
			break
		}

		entry, deserErr := c.serializer.Deserialize(data)
		if deserErr != nil {
			return 0, nil, deserErr
		}

		entryName, _ := entry.Value(0).AsVarchar()
		if entryName != name {
			continue
		}

		pageIdVal, _ := entry.Value(1).AsBigInt()
		schemaBytesVal, _ := entry.Value(2).AsVarchar()

		userSchema, err := tuple.DeserializeSchema([]byte(schemaBytesVal))
		if err != nil {
			return 0, nil, err
		}

		return uint16(pageIdVal), userSchema, nil
	}

	return 0, nil, ErrTableNotFound
}

// insertEntry serializes and inserts one catalog row describing a table.
func (c *Catalog) insertEntry(name string, tableHeap *tablemanager.TableHeap, userSchema *tuple.Schema) error {
	schemaBytes, err := tuple.SerializeSchema(userSchema)
	if err != nil {
		return err
	}

	row := tuple.NewTuple([]tuple.Value{
		tuple.NewVarcharValue(name),
		tuple.NewBigIntValue(int64(tableHeap.FirstPageId())),
		tuple.NewVarcharValue(string(schemaBytes)),
	})

	data, err := c.serializer.Serialize(row)
	if err != nil {
		return err
	}

	_, err = c.heap.Insert(data)
	return err
}
