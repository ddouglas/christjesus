package utils

import (
	"fmt"
	"reflect"
)

var ColumnTag = "db"

func StructTagValues(input any) []string {

	targetValue := reflect.ValueOf(input)
	if targetValue.Kind() == reflect.Ptr {
		targetValue = targetValue.Elem()
	}

	if targetValue.Kind() != reflect.Struct {
		panic("input must be a pointer to a struct or a struct")
	}

	targetType := targetValue.Type()

	result := make([]string, 0, targetValue.NumField())

	for i := 0; i < targetValue.NumField(); i++ {
		field := targetType.Field(i)
		fieldValue := targetValue.Field(i)

		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		// Handle embedded structs (Anonymous fields)
		if field.Anonymous {
			// Dereference pointer if needed
			embeddedValue := fieldValue
			if embeddedValue.Kind() == reflect.Ptr {
				if embeddedValue.IsNil() {
					// For tag extraction, we need the type even if nil
					embeddedValue = reflect.New(field.Type.Elem()).Elem()
				} else {
					embeddedValue = embeddedValue.Elem()
				}
			}

			// Recursively get tags from embedded struct
			if embeddedValue.Kind() == reflect.Struct {
				embeddedTags := StructTagValues(embeddedValue.Interface())
				result = append(result, embeddedTags...)
			}
			continue
		}

		// Handle regular fields with db tags
		tagValue := field.Tag.Get(ColumnTag)
		if tagValue == "" || tagValue == "-" {
			continue
		}

		result = append(result, tagValue)
	}

	return result

}

func StructToMap(input any) map[string]any {

	result := make(map[string]any)

	itemValue := reflect.ValueOf(input)
	if itemValue.Kind() == reflect.Ptr {
		itemValue = itemValue.Elem()
	}

	if itemValue.Kind() != reflect.Struct {
		panic("input must be a pointer to a struct or a struct")
	}

	itemType := itemValue.Type()

	for i := 0; i < itemValue.NumField(); i++ {
		field := itemType.Field(i)
		fieldValue := itemValue.Field(i)

		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		// Handle embedded structs (Anonymous fields)
		if field.Anonymous {
			// Dereference pointer if needed
			embeddedValue := fieldValue
			if embeddedValue.Kind() == reflect.Ptr {
				if embeddedValue.IsNil() {
					continue // Skip nil embedded pointers
				}
				embeddedValue = embeddedValue.Elem()
			}

			// Recursively flatten embedded struct fields
			if embeddedValue.Kind() == reflect.Struct {
				embeddedMap := StructToMap(embeddedValue.Interface())
				for k, v := range embeddedMap {
					result[k] = v
				}
			}
			continue
		}

		// Handle regular fields with db tags
		tagValue := field.Tag.Get(ColumnTag)
		if tagValue == "" || tagValue == "-" {
			continue
		}

		result[tagValue] = fieldValue.Interface()
	}

	return result

}

func ErrorWrapOrNil(err error, msg string) error {
	if err == nil {
		return nil
	}

	if msg == "" {
		return err
	}

	return fmt.Errorf("%s: %w", msg, err)

}
