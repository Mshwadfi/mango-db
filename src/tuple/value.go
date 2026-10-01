package tuple

import "errors"

var (
	// ErrTypeMismatch is returned when a value is of the wrong type
	ErrTypeMismatch = errors.New("value: type mismatch")
	errInvalidType  = errors.New("value: invalid type")
)

type Value struct {
	typ         TypeID
	intValue    int32
	boolValue   bool
	stringValue string
	bigIntValue int64
}

func NewIntValue(value int32) Value {
	return Value{
		typ:      TypeInt,
		intValue: value,
	}
}

func NewBigIntValue(value int64) Value {
	return Value{
		typ:         TypeBigInt,
		bigIntValue: value,
	}
}

func NewBooleanValue(value bool) Value {
	return Value{
		typ:       TypeBoolean,
		boolValue: value,
	}
}

func NewVarcharValue(value string) Value {
	return Value{
		typ:         TypeVarchar,
		stringValue: value,
	}
}

func (v Value) Type() TypeID {
	return v.typ
}

func (v Value) AsInt() (int32, error) {
	if v.typ != TypeInt {
		return 0, ErrTypeMismatch
	}
	return v.intValue, nil
}

func (v Value) AsBigInt() (int64, error) {
	if v.typ != TypeBigInt {
		return 0, ErrTypeMismatch
	}
	return v.bigIntValue, nil
}

func (v Value) AsBoolean() (bool, error) {
	if v.typ != TypeBoolean {
		return false, ErrTypeMismatch
	}
	return v.boolValue, nil
}

func (v Value) AsVarchar() (string, error) {
	if v.typ != TypeVarchar {
		return "", ErrTypeMismatch
	}
	return v.stringValue, nil
}
