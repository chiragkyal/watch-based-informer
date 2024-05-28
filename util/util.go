package util

import (
	"fmt"
	"reflect"
)

// ChangedField represents a field that has changed, storing its old and new values.
type ChangedField struct {
	Field  string
	OldVal interface{}
	NewVal interface{}
}

// GetChangedStatusFields compares the Status fields of two Kubernetes resources and returns a slice
// of ChangedField structs representing the fields that have been modified.
func GetChangedStatusFields(oldObj, newObj interface{}) []ChangedField {
	changedFields := []ChangedField{}

	oldVal := reflect.ValueOf(oldObj).Elem()
	newVal := reflect.ValueOf(newObj).Elem()

	for i := 0; i < oldVal.NumField(); i++ {
		fieldName := oldVal.Type().Field(i).Name
		if fieldName == "Status" {
			oldStatus := oldVal.Field(i)
			newStatus := newVal.Field(i)

			// Recursively compare fields within Status
			compareFields(&changedFields, "", oldStatus, newStatus)
		}
	}

	return changedFields
}

// compareFields recursively compares fields, storing changed fields with their old and new values.
func compareFields(changedFields *[]ChangedField, prefix string, oldVal, newVal reflect.Value) {
	if oldVal.Kind() != newVal.Kind() {
		// Types are different, record the change
		*changedFields = append(*changedFields, ChangedField{
			Field:  prefix,
			OldVal: oldVal.Interface(),
			NewVal: newVal.Interface(),
		})
		return
	}

	switch oldVal.Kind() {
	case reflect.Struct:
		for i := 0; i < oldVal.NumField(); i++ {
			fieldName := oldVal.Type().Field(i).Name
			newPrefix := fieldName
			if prefix != "" {
				newPrefix = fmt.Sprintf("%s.%s", prefix, fieldName)
			}
			compareFields(changedFields, newPrefix, oldVal.Field(i), newVal.Field(i))
		}
	case reflect.Slice:
		// Handle slices (you might want to customize how slice changes are recorded)
		if oldVal.Len() != newVal.Len() {
			*changedFields = append(*changedFields, ChangedField{
				Field:  prefix,
				OldVal: oldVal.Interface(),
				NewVal: newVal.Interface(),
			})
		} else {
			for i := 0; i < oldVal.Len(); i++ {
				newPrefix := fmt.Sprintf("%s[%d]", prefix, i)
				compareFields(changedFields, newPrefix, oldVal.Index(i), newVal.Index(i))
			}
		}
	case reflect.Map:
		// Handle maps (customize as needed)
		if oldVal.Len() != newVal.Len() {
			*changedFields = append(*changedFields, ChangedField{
				Field:  prefix,
				OldVal: oldVal.Interface(),
				NewVal: newVal.Interface(),
			})
		} else {
			for _, key := range oldVal.MapKeys() {
				newPrefix := fmt.Sprintf("%s[%v]", prefix, key.Interface())
				compareFields(changedFields, newPrefix, oldVal.MapIndex(key), newVal.MapIndex(key))
			}
		}
	default:
		if !reflect.DeepEqual(oldVal.Interface(), newVal.Interface()) {
			*changedFields = append(*changedFields, ChangedField{
				Field:  prefix,
				OldVal: oldVal.Interface(),
				NewVal: newVal.Interface(),
			})
		}
	}
}
