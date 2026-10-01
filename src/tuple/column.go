package tuple

import "errors"

var ErrInvalidColumnType = errors.New("column: invalid column type")

type Column struct {
	Name string
	Type TypeID
}

// creating a new column this way Column{Name: "id", Type: TypeInt} is not recommended because it does not check if the type is valid or not
// type is correct
// so we should use NewColumn function to create a new valid column
// there is no thing in this codebase prevent us from creating column the wrong way
// and we i am sure i will do this
func NewColumn(name string, typ TypeID) (Column, error) {
	if !typ.IsValid() {
		return Column{}, ErrInvalidColumnType
	}
	return Column{
		Name: name,
		Type: typ,
	}, nil
}
