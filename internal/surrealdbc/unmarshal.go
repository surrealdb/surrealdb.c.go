package surrealdbc

import (
	"fmt"
	"reflect"
)

// UnmarshalValue converts a Value into a Go value of type T.
//
// For struct T: the Value must be ValueObject. Fields are matched by `db:"..."` tags.
// For primitive T (string, bool, int/int64, float64): the Value is extracted directly.
func UnmarshalValue[T any](v Value) (T, error) {
	var zero T
	rt := reflect.TypeOf(zero)
	if rt == nil {
		return zero, fmt.Errorf("unmarshal: cannot unmarshal into untyped interface")
	}

	isPtr := rt.Kind() == reflect.Ptr
	if isPtr {
		rt = rt.Elem()
	}

	rv := reflect.New(rt).Elem()

	if err := setReflectValue(rv, v); err != nil {
		return zero, err
	}

	if isPtr {
		ptr := reflect.New(rt)
		ptr.Elem().Set(rv)
		return ptr.Interface().(T), nil
	}
	return rv.Interface().(T), nil
}

func setReflectValue(rv reflect.Value, v Value) error {
	switch rv.Kind() {
	case reflect.String:
		switch v.tag {
		case ValueString:
			rv.SetString(v.str)
		case ValueThing:
			rv.SetString(v.thing[0] + ":" + v.thing[1])
		default:
			rv.SetString(v.String())
		}

	case reflect.Bool:
		if v.tag != ValueBool {
			return fmt.Errorf("unmarshal: expected Bool, got %s", v.tag)
		}
		rv.SetBool(v.b)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.tag != ValueNumber {
			return fmt.Errorf("unmarshal: expected Number, got %s", v.tag)
		}
		rv.SetInt(v.num)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v.tag != ValueNumber {
			return fmt.Errorf("unmarshal: expected Number, got %s", v.tag)
		}
		rv.SetUint(uint64(v.num))

	case reflect.Float32, reflect.Float64:
		if v.tag != ValueNumber {
			return fmt.Errorf("unmarshal: expected Number, got %s", v.tag)
		}
		rv.SetFloat(v.float)

	case reflect.Struct:
		if v.tag != ValueObject {
			return fmt.Errorf("unmarshal: expected Object for struct, got %s", v.tag)
		}
		return unmarshalStruct(rv, v.object)

	default:
		return fmt.Errorf("unmarshal: unsupported target type %s", rv.Kind())
	}
	return nil
}

func unmarshalStruct(rv reflect.Value, kvs []KV) error {
	rt := rv.Type()

	keyToField := make(map[string]int, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		key := fieldKey(f)
		if key == "-" {
			continue
		}
		keyToField[key] = i
	}

	for _, kv := range kvs {
		idx, ok := keyToField[kv.Key]
		if !ok {
			continue
		}
		fv := rv.Field(idx)
		if err := setReflectValue(fv, kv.Value); err != nil {
			return fmt.Errorf("field %s: %w", rt.Field(idx).Name, err)
		}
	}
	return nil
}
