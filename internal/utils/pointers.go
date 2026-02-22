package utils

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

func BoolPtr(b bool) *bool {
	return &b
}

func Float64Ptr(f float64) *float64 {
	return &f
}

func IntPtr(i int) *int {
	return &i
}

func UintPtr(i uint) *uint {
	return &i
}

func Uint64Ptr(i uint64) *uint64 {
	return &i
}

func Int32Ptr(i int32) *int32 {
	return &i
}

func In64tPtr(i int64) *int64 {
	return &i
}

func StringPtr(s string) *string {
	return &s
}

func TimePtr(t time.Time) *time.Time {
	return &t
}

func PtrTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}

	return *t
}

func PtrFloat64(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

func PtrInt64(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

func PtrUint64(i *uint64) uint64 {
	if i == nil {
		return 0
	}
	return *i
}

func PtrUint(i *uint) uint {
	if i == nil {
		return 0
	}
	return *i
}

func PtrInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

func PtrInt32(i *int32) int32 {
	if i == nil {
		return 0
	}
	return *i
}

func PtrString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func RoundFloat64(f float64, places int) float64 {
	factor := math.Pow(10, float64(places))
	return math.Round(f*factor) / factor
}

func WrapError(err error, msg string) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", msg, err)
}

func WrapErrorf(err error, msg string, args ...any) error {
	return WrapError(err, fmt.Sprintf(msg, args...))
}

func MustMarshalJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Errorf("failed to marshal to JSON: %w", err))
	}
	return data
}

const columnPrefixFmt = "%s.%s"

func PrefixSliceOfStrings(prefix string, input []string, ignore ...string) []string {
	out := make([]string, len(input))

inputloop:
	for i, v := range input {
		for _, ignored := range ignore {
			if v == ignored {
				continue inputloop
			}
		}

		out[i] = fmt.Sprintf(columnPrefixFmt, prefix, v)
	}
	return out
}

func FilterSliceString(slice []string, filter string) []string {
	var out = make([]string, 0, len(slice))
	for _, v := range slice {
		if v == filter {
			continue
		}
		out = append(out, v)
	}
	return out
}
