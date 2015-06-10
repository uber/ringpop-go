package event

import (
	"reflect"
	"testing"
	"time"
)

func TestPrecisionTimingUpdate(t *testing.T) {
	e1 := NewPrecisionTiming("test", 5*time.Microsecond)
	e2 := NewPrecisionTiming("test", 3*time.Microsecond)
	e3 := NewPrecisionTiming("test", 7*time.Microsecond)
	e1.Update(e2)
	e1.Update(e3)

	expected := []string{"test.count:3|a", "test.avg:0.005000|ms", "test.min:0.003000|ms", "test.max:0.007000|ms"}
	actual := e1.Stats()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("did not receive all metrics: Expected: %T %v, Actual: %T %v ", expected, expected, actual, actual)
	}
}
