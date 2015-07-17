package event

import (
	"reflect"
	"testing"
)

func TestTimingUpdate(t *testing.T) {
	e1 := NewTiming("test", 5)
	e2 := NewTiming("test", 3)
	e3 := NewTiming("test", 7)
	e1.Update(e2)
	e1.Update(e3)

	expected := []string{"test.count:3|a", "test.avg:5|ms", "test.min:3|ms", "test.max:7|ms"}
	actual := e1.Stats()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("did not receive all metrics: Expected: %T %v, Actual: %T %v ", expected, expected, actual, actual)
	}
}
