package util

import (
	"errors"
	"fmt"
	"reflect"
)

var (
	ReflectString = reflect.TypeOf("")
	ReflectInt    = reflect.TypeOf(0)
	ReflectBool   = reflect.TypeOf(false)
)

func checkAttr(m map[string]any, attr string, aType reflect.Type) error {
	// exist?
	if _, exist := m[attr]; !exist {
		return ErrLost{attr}
	}
	// valid type?
	if attrType := reflect.TypeOf(m[attr]); attrType.Kind() != aType.Elem().Kind() {
		return ErrInvalid{attr}
	}
	return nil
}

func MustHave(m map[string]any, attrList map[string]any) error {
	for key, value := range attrList {
		rValue := reflect.ValueOf(value)
		if err := checkAttr(m, key, rValue.Type()); err != nil {
			return err
		}
		if rValue.Elem().Kind() == reflect.Slice {
			eType := rValue.Elem().Type().Elem()
			for _, v := range m[key].([]any) {
				rValue.Elem().Set(
					reflect.Append(rValue.Elem(),
						reflect.ValueOf(v).Convert(eType)))
			}
			continue
		}
		rValue.Elem().
			Set(reflect.ValueOf(m[key]).
				Convert(rValue.Elem().Type()))
	}
	return nil
}

func MayHave(m map[string]any, attrList map[string]any) error {
	for key, value := range attrList {
		rValue := reflect.ValueOf(value)
		if err := checkAttr(m, key, rValue.Type()); err != nil {
			if errors.Is(err, ErrLost{Attr: key}) {
				reflect.ValueOf(value).Elem().SetZero()
			} else {
				return err
			}
		} else {
			rValue.Elem().
				Set(reflect.ValueOf(m[key]).
					Convert(rValue.Elem().Type()))
		}
	}
	return nil
}

type ErrLost struct {
	Attr string
}

func (e ErrLost) Error() string {
	return fmt.Sprintf("config: lost attribute '%v'", e.Attr)
}

func (e ErrLost) Is(err error) bool {
	if _, ok := err.(ErrLost); !ok {
		return false
	}
	return e.Attr == err.(ErrLost).Attr
}

type ErrInvalid struct {
	Attr string
}

func (e ErrInvalid) Error() string {
	return fmt.Sprintf("config: invalid attribute '%v'", e.Attr)
}

func (e ErrInvalid) Is(err error) bool {
	if _, ok := err.(ErrInvalid); !ok {
		return false
	}
	return e.Attr == err.(ErrInvalid).Attr
}
