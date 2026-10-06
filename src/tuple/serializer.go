package tuple

import (
	"encoding/binary"
	"errors"
	"fmt"
)

var (
	ErrNilSchema   = errors.New("serializer: schema is nil")
	ErrCorruptData = errors.New("serializer: corrupt or truncated tuple data")
)

// columnLayout is the precomputed position of one column within a tuple's
// fixed-size region.
type columnLayout struct {
	offset int
	width  int
}

type schemaLayout struct {
	columns   []columnLayout
	fixedSize int
	bitmapLen int
}

func computeLayout(schema *Schema) schemaLayout {
	n := schema.GetNumberOfColumns()
	columns := make([]columnLayout, n)

	offset := 0
	for i := 0; i < n; i++ {
		col, _ := schema.GetColumn(i)
		width := fixedWidth(col.Type)
		columns[i] = columnLayout{offset: offset, width: width}
		offset += width
	}

	return schemaLayout{
		columns:   columns,
		fixedSize: offset,
		bitmapLen: (n + 7) / 8,
	}
}

func fixedWidth(t TypeID) int {
	switch t {
	case TypeInt:
		return 4
	case TypeBigInt:
		return 8
	case TypeBoolean:
		return 1
	case TypeVarchar:
		return 4 // 2-byte offset + 2-byte length
	default:
		return 0
	}
}

// Serializer converts Tuples to and from []byte for one fixed Schema. The
// schema's layout is computed once, at construction, and reused for every
// Serialize/Deserialize call — the whole reason this is a struct rather
// than a pair of free functions.

// |id|name|age|active| => bytes: [bitmap][int32][string metadata (offset, length)][bool][string bytes]
type Serializer struct {
	schema *Schema
	layout schemaLayout
}

func NewSerializer(schema *Schema) (*Serializer, error) {
	if schema == nil {
		return nil, ErrNilSchema
	}
	return &Serializer{
		schema: schema,
		layout: computeLayout(schema),
	}, nil
}

// Serialize converts tuple into bytes laid out as:
//
//	[ NULL bitmap ][ fixed-size region ][ variable-size region ]
//
// tuple must match the serializer's schema

func (s *Serializer) Serialize(t *Tuple) ([]byte, error) {
	if err := t.ValidateAgainst(s.schema); err != nil {
		return nil, err
	}

	varSize := 0
	for i := 0; i < t.NumValues(); i++ {
		col, _ := s.schema.GetColumn(i)
		if col.Type == TypeVarchar {
			str, _ := t.Value(i).AsVarchar()
			varSize += len(str)
		}
	}

	totalSize := s.layout.bitmapLen + s.layout.fixedSize + varSize
	buf := make([]byte, totalSize)

	// bitmap already zero-valued by make([]byte, ...) — nothing to write
	// yet, since nothing is nullable. Left as an explicit step for clarity
	// and as the hook point for future NULL support.

	fixedStart := s.layout.bitmapLen
	varCursor := s.layout.bitmapLen + s.layout.fixedSize // start of variable region

	// write every value into its slot.
	for i := 0; i < t.NumValues(); i++ {
		col, _ := s.schema.GetColumn(i)
		slot := s.layout.columns[i]
		slotStart := fixedStart + slot.offset
		v := t.Value(i)

		switch col.Type {
		case TypeInt:
			val, _ := v.AsInt()
			binary.LittleEndian.PutUint32(buf[slotStart:slotStart+4], uint32(val))

		case TypeBigInt:
			val, _ := v.AsBigInt()
			binary.LittleEndian.PutUint64(buf[slotStart:slotStart+8], uint64(val))

		case TypeBoolean:
			val, _ := v.AsBoolean()
			if val {
				buf[slotStart] = 1
			} else {
				buf[slotStart] = 0
			}

		case TypeVarchar:
			str, _ := v.AsVarchar()
			n := len(str)
			copy(buf[varCursor:varCursor+n], str)
			binary.LittleEndian.PutUint16(buf[slotStart:slotStart+2], uint16(varCursor))
			binary.LittleEndian.PutUint16(buf[slotStart+2:slotStart+4], uint16(n))
			varCursor += n
		}
	}

	return buf, nil
}

// Deserialize converts bytes previously produced by Serialize (using the
// same schema) back into a Tuple. It bounds-checks every offset/length it
// reads before trusting it, to avoid panicking on corrupt or truncated
// input.

// byte[] => |bitmap|fixed region|variable region| => Tuple
func (s *Serializer) Deserialize(data []byte) (*Tuple, error) {
	minSize := s.layout.bitmapLen + s.layout.fixedSize
	if len(data) < minSize {
		return nil, fmt.Errorf("%w: buffer smaller than fixed region (%d < %d)",
			ErrCorruptData, len(data), minSize)
	}

	bitmap := data[:s.layout.bitmapLen]
	fixedStart := s.layout.bitmapLen

	values := make([]Value, s.schema.GetNumberOfColumns())

	for i := 0; i < s.schema.GetNumberOfColumns(); i++ {
		if isNull(bitmap, i) {
			// no null support yet
			continue
		}

		col, _ := s.schema.GetColumn(i)
		slot := s.layout.columns[i]
		slotStart := fixedStart + slot.offset
		slotEnd := slotStart + slot.width

		if slotEnd > len(data) {
			return nil, fmt.Errorf("%w: column %q slot out of bounds",
				ErrCorruptData, col.Name)
		}

		switch col.Type {
		case TypeInt:
			v := int32(binary.LittleEndian.Uint32(data[slotStart:slotEnd]))
			values[i] = NewIntValue(v)

		case TypeBigInt:
			v := int64(binary.LittleEndian.Uint64(data[slotStart:slotEnd]))
			values[i] = NewBigIntValue(v)

		case TypeBoolean:
			values[i] = NewBooleanValue(data[slotStart] == 1)

		case TypeVarchar:
			strOffset := int(binary.LittleEndian.Uint16(data[slotStart : slotStart+2]))
			strLen := int(binary.LittleEndian.Uint16(data[slotStart+2 : slotStart+4]))

			if strOffset < 0 || strLen < 0 || strOffset+strLen > len(data) {
				return nil, fmt.Errorf("%w: column %q string out of bounds",
					ErrCorruptData, col.Name)
			}

			strBytes := make([]byte, strLen)
			copy(strBytes, data[strOffset:strOffset+strLen])
			values[i] = NewVarcharValue(string(strBytes))
		}
	}

	return NewTuple(values), nil
}

func isNull(bitmap []byte, columnIndex int) bool {
	byteIdx := columnIndex / 8
	bitIdx := uint(columnIndex % 8)
	return bitmap[byteIdx]&(1<<bitIdx) != 0
}
