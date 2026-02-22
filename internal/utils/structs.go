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

		if targetType.Field(i).PkgPath != "" {
			continue
		}

		tagValue := targetType.Field(i).Tag.Get(ColumnTag)
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

		if itemType.Field(i).PkgPath != "" {
			continue
		}

		tagValue := itemType.Field(i).Tag.Get(ColumnTag)
		if tagValue == "" || tagValue == "-" {
			continue
		}

		result[tagValue] = itemValue.Field(i).Interface()

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
