package reflectStruct

import (
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
)

func Map(x interface{}) (output map[string]interface{}, error error) {
	output = make(map[string]interface{})
	b, err := json.Marshal(x)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(b, &output)
	if err != nil {
		return nil, err
	}
	return output, nil
}

func CopyToStructFromMap(input interface{}, values map[string]interface{}) (err error) {
	if input == nil {
		return nil
	}

	inputStruct := reflect.TypeOf(input)
	if inputStruct.Kind() == reflect.Ptr {
		inputStruct = inputStruct.Elem()
	}

	inputValue := reflect.ValueOf(input)
	if inputValue.Kind() == reflect.Ptr {
		inputValue = inputValue.Elem()
	}

	for k, v := range values {

		field, present := inputStruct.FieldByName(k)
		if !present {
			continue
		}

		if !field.IsExported() {
			continue
		}

		valueType := reflect.TypeOf(v)
		if valueType.Kind() == reflect.Ptr {
			valueType = valueType.Elem()
		}

		ves := reflect.ValueOf(v)
		if ves.Kind() == reflect.Ptr {
			ves = ves.Elem()
		}

		fieldVal := inputValue.FieldByName(field.Name)
		if fieldVal.Kind() == reflect.Ptr {
			fieldVal = fieldVal.Elem()
		}

		if inputStruct != valueType {
			switch field.Type.Kind() {
			case reflect.Bool:
				switch valueType.Kind() {
				case reflect.String:
					kv, err := strconv.ParseBool(ves.Interface().(string))
					if err != nil {
						return err
					}
					fieldVal.SetBool(kv)
				default:
					return errors.New("Environment variable " + k + " is not a bool")
				}
			case reflect.Int:
				switch valueType.Kind() {
				case reflect.Int:
					fieldVal.SetInt(ves.Int())
				case reflect.Int64:
					fieldVal.SetInt(ves.Int())
				case reflect.Float64:
					fieldVal.SetInt(int64(ves.Float()))
				case reflect.String:
					kv, err := strconv.ParseInt(ves.Interface().(string), 10, 64)
					if err != nil {
						return err
					}
					fieldVal.SetInt(kv)
				default:
					return errors.New("Environment variable " + k + " is not a string")
				}
			case reflect.Int64:
				switch valueType.Kind() {
				case reflect.Int64:
					fieldVal.SetInt(ves.Int())
				case reflect.Float64:
					fieldVal.SetInt(int64(ves.Float()))
				case reflect.String:
					kv, err := strconv.ParseInt(ves.Interface().(string), 10, 64)
					if err != nil {
						return err
					}
					fieldVal.SetInt(kv)
				default:
					return errors.New("Environment variable " + k + " is not a string")
				}
			case reflect.Float64:
				switch valueType.Kind() {
				case reflect.Int:
					fieldVal.SetFloat(float64(ves.Int()))
				case reflect.Int64:
					fieldVal.SetFloat(float64(ves.Int()))
				case reflect.Float64:
					fieldVal.SetFloat(ves.Float())
				case reflect.String:
					kv, err := strconv.ParseFloat(ves.Interface().(string), 64)
					if err != nil {
						return err
					}
					fieldVal.SetFloat(kv)
				default:
					return errors.New("Environment variable " + k + " is not a float64")
				}
			case reflect.Ptr:

			case reflect.Map:
				switch valueType.Kind() {
				case reflect.Map:
					fieldVal.Set(ves)
				default:
					return errors.New("Environment variable " + k + " is not a float64")
				}
			case reflect.Struct:
				switch valueType.Kind() {
				case reflect.Struct:
					fieldVal.Set(ves)
					return CopyStruct(fieldVal.Addr().Interface(), ves.Interface())
				case reflect.Map:
					return CopyToStructFromMap(fieldVal.Interface(), ves.Interface().(map[string]interface{}))
				default:
					return errors.New("Environment variable " + k + " is not a float64")
				}
			case reflect.String:
				switch valueType.Kind() {
				case reflect.Int:
					fieldVal.SetString(strconv.Itoa(int(ves.Int())))
				case reflect.Int64:
					fieldVal.SetString(strconv.FormatInt(ves.Int(), 10))
				case reflect.Float64:
					fieldVal.SetString(strconv.FormatFloat(ves.Float(), 'g', -1, 64))
				case reflect.String:
					fieldVal.SetString(ves.String())
				default:
					return errors.New("Environment variable " + k + " is not a string")
				}
			default:

			}
		}
	}
	return nil
}

func convert(field reflect.Value, value reflect.Value) (err error) {
	switch field.Kind() {
	case reflect.Bool:
		if value.Kind() == reflect.String {
			kv, err := strconv.ParseBool(value.Interface().(string))
			if err != nil {
				return err
			}
			field.SetBool(kv)
		}
	case reflect.Int, reflect.Int64:
		if value.Kind() == reflect.String {
			kv, err := strconv.ParseInt(value.Interface().(string), 10, 64)
			if err != nil {
				return err
			}
			field.SetInt(kv)
		}
	case reflect.Float64:
		if value.Kind() == reflect.String {
			kv, err := strconv.ParseFloat(value.Interface().(string), 64)
			if err != nil {
				return err
			}
			field.SetFloat(kv)
		}
	}
	return nil
}

func CopyStruct(copyFrom, copyTo interface{}) error {
	x, err := Map(copyFrom)
	if err != nil {
		return err
	}

	return CopyToStructFromMap(copyTo, x)
}

func FindFields(x interface{}, from map[string]interface{}) {
	inputType := reflect.TypeOf(x)
	if inputType.Kind() == reflect.Ptr {
		inputType = inputType.Elem()
	}

	inputValue := reflect.ValueOf(x)
	if inputValue.Kind() == reflect.Ptr {
		inputValue = inputValue.Elem()
	}

	for i := 0; i < inputValue.NumField(); i++ {
		field := inputValue.Field(i)
		if field.Kind() == reflect.Pointer {
			field = field.Elem()
		}

		if !field.IsValid() {
			field = reflect.New(inputType.Field(i).Type.Elem()).Elem()
		}

		if field.Kind() == reflect.Struct {
			nv := reflect.New(inputType.Field(i).Type.Elem())
			inputValue.Field(i).Set(nv)
			FindFields(inputValue.Field(i).Interface(), from)
		}

		if field.Kind() == reflect.Slice {
			for j := 0; j < field.Len(); j++ {
				FindFields(field.Index(j), from)
			}
		}

		ts := inputType.Field(i).Tag.Get("json")
		parts := strings.Split(ts, ",")
		fv, _ := from[parts[len(parts)-1]]

		err := convert(field, reflect.ValueOf(fv))
		if err != nil {
			panic(err)
		}
	}
}
