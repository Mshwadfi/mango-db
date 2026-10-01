package tuple

// this is our db type system
// TypeID defines a db level type not a standard GOlang type
type TypeID uint8

const (
	TypeUnknown TypeID = iota
	TypeInt
	TypeBigInt
	TypeBoolean
	TypeVarchar
)

// checks whether the type is valid or not
func (t TypeID) IsValid() bool {
	return t >= TypeInt && t <= TypeVarchar
}

// map type id to human readable type string that is used for error messages and debugging
/** "expected type int got type vachar"
** 	vs
** 	"expected type 0 got type 3"
 */
func (t TypeID) String() string {
	switch t {
	case TypeInt:
		return "INT"
	case TypeBigInt:
		return "BIGINT"
	case TypeBoolean:
		return "BOOLEAN"
	case TypeVarchar:
		return "VARCHAR"
	}
	return "UNKNOWN"
}
