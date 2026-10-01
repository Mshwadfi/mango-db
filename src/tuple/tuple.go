package tuple

import (
	"errors"
	"fmt"
)

var (
	ErrValueCountMismatch = errors.New("tuple: value count does not match schema column count")
	ErrValueTypeMismatch  = errors.New("tuple: value type does not match schema column type")
)

// tuple is our db row
type Tuple struct {
	values []Value
}

func NewTuple(values []Value) *Tuple {
	owned := make([]Value, len(values))
	copy(owned, values)
	return &Tuple{values: owned}
}

func (t *Tuple) NumValues() int {
	return len(t.values)
}

func (t *Tuple) Value(i int) Value {
	return t.values[i]
}

// ValidateAgainst checks that this tuple's shape matches schema: same
// number of values, and each value's type matches its corresponding
// column's declared type, position by position.
func (t *Tuple) ValidateAgainst(schema *Schema) error {
	if len(t.values) != schema.GetNumberOfColumns() {
		return fmt.Errorf("%w: tuple has %d values, schema has %d columns",
			ErrValueCountMismatch, len(t.values), schema.GetNumberOfColumns())
	}

	for i, v := range t.values {
		col, err := schema.GetColumn(i)
		if err != nil {
			return err
		}
		if v.Type() != col.Type {
			return fmt.Errorf("%w: column %q expects %s, got %s",
				ErrValueTypeMismatch, col.Name, col.Type, v.Type())
		}
	}

	return nil
}
