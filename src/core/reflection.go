package core

import "reflect"

func ReflectionCallFunction(obj interface{}, funcName string, paramValues []reflect.Value) {
	chainObjReflect := reflect.ValueOf(obj) // Call function via reflection
	method := chainObjReflect.MethodByName(funcName)
	if !method.IsValid() {
		LogError("Could not call the blockchain object to cache posts")
		return
	}
	method.Call(paramValues) // Pass the database object to the reflectively called function
}
func ReflectionIsPopulated(obj interface{}) bool {
	return !reflect.DeepEqual(obj, reflect.Zero(reflect.TypeOf(obj)).Interface())
}
func ReflectionTypeOf(obj interface{}) string {
	return reflect.TypeOf(obj).String()
}
func ReflectionValueOf(obj interface{}, fieldName string) interface{} {
	reflectValue := reflect.ValueOf(obj)
	if reflectValue.Kind() == reflect.Ptr { // check if object is a pointer, if so, get the value
		reflectValue = reflectValue.Elem()
	}
	if reflectValue.Kind() == reflect.Struct { // check if object is a struct
		field := reflectValue.FieldByName(fieldName)
		if field.IsValid() {
			return field.Interface()
		}
	}
	return nil
}
