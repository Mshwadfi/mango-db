package tuple

import (
	"encoding/binary"
	"errors"
)

var ErrCorruptSchema = errors.New("tuple: corrupt or truncated schema data")

// serialize function serializes a schema into a byte slice. The format is as follows:
// | 2 bytes: number of columns (uint16)| for each column: | 2 bytes: length of column name (uint16) | column name bytes | 1 byte: column type (TypeID) |

func SerializeSchema(schema *Schema) ([]byte, error) {
	if schema == nil {
		return nil, ErrNilSchema
	}

	n := schema.GetNumberOfColumns()
	size := 2
	for i := 0; i < n; i++ {
		col, _ := schema.GetColumn(i) // safe: i always in range
		size += 2 + len(col.Name) + 1
	}

	data := make([]byte, size)
	binary.LittleEndian.PutUint16(data[0:2], uint16(n))

	offset := 2
	for i := 0; i < n; i++ {
		col, _ := schema.GetColumn(i)
		nameLen := uint16(len(col.Name))
		binary.LittleEndian.PutUint16(data[offset:offset+2], nameLen)
		offset += 2
		copy(data[offset:offset+int(nameLen)], col.Name)
		offset += int(nameLen)
		data[offset] = byte(col.Type)
		offset++
	}

	return data, nil
}

func DeserializeSchema(data []byte) (*Schema, error) {
	if len(data) < 2 {
		return nil, ErrCorruptSchema
	}

	numColumns := binary.LittleEndian.Uint16(data[0:2])
	offset := 2

	columns := make([]Column, numColumns)

	for i := 0; i < int(numColumns); i++ {
		if offset+2 > len(data) {
			return nil, ErrCorruptSchema
		}

		nameLen := binary.LittleEndian.Uint16(data[offset : offset+2])
		offset += 2

		if offset+int(nameLen)+1 > len(data) {
			return nil, ErrCorruptSchema
		}

		name := string(data[offset : offset+int(nameLen)])
		offset += int(nameLen)

		colType := TypeID(data[offset])
		offset++

		if !colType.IsValid() {
			return nil, ErrInvalidColumnType
		}

		columns[i] = Column{Name: name, Type: colType}
	}

	return NewSchema(columns)
}
