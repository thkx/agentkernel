package state

import "reflect"

// CloneValue performs a best-effort deep clone for common mutable container
// types used in state payloads so snapshots do not share map/slice internals.
func CloneValue(value any) any {
	cloned := cloneRecursive(reflect.ValueOf(value))
	if !cloned.IsValid() {
		return nil
	}
	return cloned.Interface()
}

func cloneRecursive(value reflect.Value) reflect.Value {
	if !value.IsValid() {
		return value
	}

	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return value
		}
		cloned := cloneRecursive(value.Elem())
		boxed := reflect.New(value.Elem().Type()).Elem()
		boxed.Set(cloned)
		return boxed
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		clonedElem := cloneRecursive(value.Elem())
		ptr := reflect.New(value.Type().Elem())
		ptr.Elem().Set(clonedElem)
		return ptr
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			cloned.SetMapIndex(cloneRecursive(iter.Key()), cloneRecursive(iter.Value()))
		}
		return cloned
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			cloned.Index(i).Set(cloneRecursive(value.Index(i)))
		}
		return cloned
	case reflect.Array:
		cloned := reflect.New(value.Type()).Elem()
		for i := 0; i < value.Len(); i++ {
			cloned.Index(i).Set(cloneRecursive(value.Index(i)))
		}
		return cloned
	case reflect.Struct:
		cloned := reflect.New(value.Type()).Elem()
		for i := 0; i < value.NumField(); i++ {
			field := cloned.Field(i)
			if !field.CanSet() {
				continue
			}
			field.Set(cloneRecursive(value.Field(i)))
		}
		return cloned
	default:
		return value
	}
}
