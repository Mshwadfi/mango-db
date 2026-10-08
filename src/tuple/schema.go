package tuple

import "errors"

var (
	ErrEmptySchema     = errors.New("schema: empty schema")
	ErrDuplicateColumn = errors.New("schema: duplicate column name")
	ErrColumnNotFound  = errors.New("schema: column not found")
)

type Schema struct {
	Columns []Column
}

func NewSchema(columns []Column) (*Schema, error) {

	if len(columns) == 0 {
		return nil, ErrEmptySchema
	}

	// check for duplicate column names
	seen := make(map[string]bool, len(columns))

	for _, col := range columns {
		if !col.Type.IsValid() {
			return nil, ErrInvalidColumnType
		}
		if seen[col.Name] {
			return nil, ErrDuplicateColumn
		}
		seen[col.Name] = true
	}

	// copy the columns that the caller sends and return a new one with another ref
	schemaColumns := make([]Column, len(columns))
	copy(schemaColumns, columns)

	return &Schema{
		Columns: schemaColumns,
	}, nil
}

// schema helpers
func (s *Schema) GetColumnIndex(name string) (int, error) {
	for i, col := range s.Columns {
		if col.Name == name {
			return i, nil
		}
	}
	return -1, ErrColumnNotFound
}

func (s *Schema) GetColumn(idx int) (Column, error) {
	if idx < 0 || idx >= len(s.Columns) {
		return Column{}, ErrColumnNotFound
	}
	return s.Columns[idx], nil
}

func (s *Schema) GetNumberOfColumns() int {
	return len(s.Columns)
}
