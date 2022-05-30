package assert

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Error asserts err is not nil
func Error(tb testing.TB, err error) bool {
	tb.Helper()
	if err == nil {
		tb.Error("Expected error, got nil")
		return false
	}
	return true
}

// NoError asserts err is nil
func NoError(tb testing.TB, err error) bool {
	tb.Helper()
	if err != nil {
		tb.Errorf("Unexpected error: %+v", err)
		return false
	}
	return true
}

// Zero asserts value is the zero value
func Zero(tb testing.TB, value interface{}) bool {
	tb.Helper()
	if !reflect.ValueOf(value).IsZero() {
		tb.Errorf("Value should be zero, got: %#v", value)
		return false
	}
	return true
}

// NotZero asserts value is not the zero value
func NotZero(tb testing.TB, value interface{}) bool {
	tb.Helper()
	if value == nil || reflect.ValueOf(value).IsZero() {
		tb.Error("Value should not be zero")
		return false
	}
	return true
}

// Equal asserts actual is equal to expected
func Equal(tb testing.TB, expected, actual interface{}) bool {
	tb.Helper()
	if !reflect.DeepEqual(expected, actual) {
		tb.Errorf("%+v != %+v\nExpected: %#v\nActual:   %#v", expected, actual, expected, actual)
		return false
	}
	return true
}

// NotEqual asserts actual is not equal to expected
func NotEqual(tb testing.TB, expected, actual interface{}) bool {
	tb.Helper()
	if reflect.DeepEqual(expected, actual) {
		tb.Errorf("Should not be equal: %+v\nExpected: %#v\nActual:    %#v", actual, expected, actual)
		return false
	}
	return true
}

func contains(tb testing.TB, collection, item interface{}) bool {
	collectionVal := reflect.ValueOf(collection)
	switch collectionVal.Kind() {
	case reflect.Slice:
		length := collectionVal.Len()
		for i := 0; i < length; i++ {
			candidateItem := collectionVal.Index(i).Interface()
			if reflect.DeepEqual(candidateItem, item) {
				return true
			}
		}
		return false
	case reflect.String:
		itemVal := reflect.ValueOf(item)
		if itemVal.Kind() != reflect.String {
			tb.Errorf("Invalid item type for string collection. Expected string, got: %T", item)
			return false
		}
		return strings.Contains(collection.(string), item.(string))
	default:
		tb.Errorf("Invalid collection type. Expected slice, got: %T", collection)
		return false
	}
}

// Contains asserts item is contained by collection
func Contains(tb testing.TB, collection, item interface{}) bool {
	tb.Helper()

	if !contains(tb, collection, item) {
		tb.Errorf("Collection does not contain expected item:\nCollection: %#v\nExpected item: %#v", collection, item)
		return false
	}
	return true
}

// NotContains asserts item is not contained by collection
func NotContains(tb testing.TB, collection, item interface{}) bool {
	tb.Helper()

	if contains(tb, collection, item) {
		tb.Errorf("Collection contains unexpected item:\nCollection: %#v\nUnexpected item: %#v", collection, item)
		return false
	}
	return true
}

// Eventually asserts fn() returns true within totalWait time, checking at the given interval
func Eventually(tb testing.TB, fn func(context.Context) bool, totalWait time.Duration, checkInterval time.Duration) bool {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), totalWait)
	defer cancel()
	for {
		success := fn(ctx)
		if success {
			return true
		}
		timer := time.NewTimer(checkInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			tb.Errorf("Condition did not become true within %s", totalWait)
			return false
		case <-timer.C:
		}
	}
}

// Panics asserts fn() panics
func Panics(tb testing.TB, fn func()) (panicked bool) {
	tb.Helper()
	defer func() {
		val := recover()
		if val != nil {
			panicked = true
		} else {
			tb.Error("Function should panic")
		}
	}()
	fn()
	return false
}

// NotPanics asserts fn() does not panic
func NotPanics(tb testing.TB, fn func()) bool {
	tb.Helper()
	defer func() {
		val := recover()
		if val != nil {
			tb.Errorf("Function should not panic, got: %#v", val)
		}
	}()
	fn()
	return true
}

// IsType asserts both value's types are equal
func IsType(tb testing.TB, expected, actual interface{}) bool {
	tb.Helper()
	expectedType := reflect.TypeOf(expected)
	actualType := reflect.TypeOf(actual)
	return Equal(tb, expectedType, actualType)
}

// Prefix asserts actual starts with expected
func Prefix(tb testing.TB, expected, actual string) bool {
	tb.Helper()
	if !strings.HasPrefix(actual, expected) {
		tb.Errorf("Actual is not prefixed with expected:\nExpected: %s\nActual:   %s", expected, actual)
		return false
	}
	return true
}

// Subset asserts expected is a subset of actual
func Subset(tb testing.TB, expected, actual interface{}) bool {
	tb.Helper()

	if !subset(tb, expected, actual) {
		tb.Errorf("Expected is not a subset of actual:\nExpected: %#v\nActual:   %#v", expected, actual)
		return false
	}
	return true
}

func subset(tb testing.TB, expected, actual interface{}) bool {
	expectedVal := reflect.ValueOf(expected)
	actualVal := reflect.ValueOf(actual)
	if expectedVal.Kind() != actualVal.Kind() {
		return false
	}
	switch expectedVal.Kind() {
	case reflect.Map:
		iter := expectedVal.MapRange()
		for iter.Next() {
			expectedKey, expectedValue := iter.Key(), iter.Value()
			actualValue := actualVal.MapIndex(expectedKey)
			if actualValue == (reflect.Value{}) || !reflect.DeepEqual(expectedValue.Interface(), actualValue.Interface()) {
				return false
			}
		}
		return true
	case reflect.Slice:
		length := expectedVal.Len()
		for i := 0; i < length; i++ {
			expectedValue := expectedVal.Index(i).Interface()
			if !contains(tb, actual, expectedValue) {
				return false
			}
		}
		return true
	default:
		tb.Errorf("Invalid subset type. Expected slice, got: %T", expected)
		return false
	}
}

func ErrorIs(tb testing.TB, target, err error) bool {
	tb.Helper()
	if errors.Is(err, target) {
		return true
	}
	tb.Errorf("Error must match target:\nExpected: %s\nActual:   %s", target, err)
	return false
}
