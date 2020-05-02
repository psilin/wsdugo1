package main

import (
	"fmt"
	"reflect"
)

func inner(data, out reflect.Value) error {
	switch out.Kind() {
	case reflect.Bool:
		//fmt.Printf("O %s %t %s %t\n", out.Type(), out.Bool(), out.Type().Name(), out.CanAddr())
		//fmt.Printf("D %s %t %s %t\n", data.Type(), data.Interface().(bool), data.Type().Name(), data.CanAddr())
		b, ok := data.Interface().(bool)
		if !ok {
			return fmt.Errorf("can not convert to bool")
		}
		out.SetBool(b)

	case reflect.Int:
		//fmt.Printf("O %s %d %s %t\n", out.Type(), out.Int(), out.Type().Name(), out.CanAddr())
		//fmt.Printf("D %s %f %s %t\n", data.Type(), data.Interface().(float64), data.Type().Name(), data.CanAddr())
		f, ok := data.Interface().(float64)
		if !ok {
			return fmt.Errorf("can not convert to float")
		}
		out.SetInt(int64(f))

	case reflect.String:
		//fmt.Printf("O %s %s %s %t\n", out.Type(), out.String(), out.Type().Name(), out.CanAddr())
		//fmt.Printf("D %s %s %s %t\n", data.Type(), data.Interface().(string), data.Type().Name(), data.CanAddr())
		s, ok := data.Interface().(string)
		if !ok {
			return fmt.Errorf("can not convert to string")
		}
		out.SetString(s)

	case reflect.Slice:
		//fmt.Printf("O %s %d %s %t\n", out.Type(), out.Len(), out.Type().Name(), out.CanAddr())
		//fmt.Printf("D %s %d %s %t\n", data.Type(), data.Len(), data.Type().Name(), data.CanAddr())
		sl, ok := data.Interface().([]interface{})
		if !ok {
			return fmt.Errorf("can not convert to slice")
		}

		//fmt.Printf("SLICE LEN: %d\n", len(sl))
		for i := 0; i < len(sl); i++ {
			d := reflect.ValueOf(sl[i])
			elem := reflect.Indirect(reflect.New(out.Type().Elem()))
			err := inner(d, elem)
			if err != nil {
				return err
			}
			out.Set(reflect.Append(out, elem))
		}

	case reflect.Struct:
		//fmt.Printf("O %s %d %s %t\n", out.Type(), out.NumField(), out.Type().Name(), out.CanAddr())
		//fmt.Printf("D %s %d %s %t\n", data.Type(), len(data.MapKeys()), data.Type().Name(), data.CanAddr())
		m, ok := data.Interface().(map[string]interface{})
		if !ok {
			return fmt.Errorf("can not convert to map")
		}

		for i := 0; i < out.NumField(); i++ {
			mm, ok := m[out.Type().Field(i).Name]
			if !ok {
				return fmt.Errorf("field %s not found", out.Type().Field(i).Name)
			}

			val := reflect.ValueOf(mm)
			//fmt.Printf("TT %s %s %s\n", out.Type().Field(i).Name, val, reflect.ValueOf(data.MapIndex(val).Interface()).Kind())
			err := inner(val, out.Field(i))
			if err != nil {
				return err
			}
		}

	default:
		return fmt.Errorf("type not found")
	}

	return nil
}

func i2s(data interface{}, out interface{}) error {
	if reflect.ValueOf(out).Kind() != reflect.Ptr {
		return fmt.Errorf("can not return result in out")
	}

	//fmt.Printf("====================================\n")
	//fmt.Printf("%s %s\n", reflect.ValueOf(data).Kind().String(), reflect.ValueOf(out).Kind().String())
	return inner(reflect.ValueOf(data), reflect.ValueOf(out).Elem())
}
