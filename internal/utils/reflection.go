package utils

import (
	"fmt"
	"reflect"
)

// CheckMissingFields checks for missing, required fields in structs
func CheckMissingFields(obj interface{}, requiredFields []string) error {
	v := reflect.ValueOf(obj).Elem()
	for _, field := range requiredFields {
		f := v.FieldByName(field)
		if !f.IsValid() {
			return fmt.Errorf("field %s is not valid", field)
		}
		zero := reflect.Zero(f.Type()).Interface()
		current := f.Interface()
		if reflect.DeepEqual(current, zero) {
			return fmt.Errorf("missing required field: %s", field)
		}
	}
	return nil
}
