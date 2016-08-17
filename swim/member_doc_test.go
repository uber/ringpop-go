package swim

import (
	"bytes"
	"fmt"
)

func ExampleMember_checksumString() {
	var b bytes.Buffer
	m := Member{
		Address:     "192.0.2.1:1234",
		Status:      Alive,
		Incarnation: 42,
	}
	m.checksumString(&b)
	fmt.Println(b.String())
	// Output: 192.0.2.1:1234alive42
}

func ExampleMember_checksumString_labels() {
	var b bytes.Buffer
	m := Member{
		Address:     "192.0.2.1:1234",
		Status:      Alive,
		Incarnation: 42,
		Labels: LabelMap{
			"hello": "world",
		},
	}
	m.checksumString(&b)
	fmt.Println(b.String())
	// Output: 192.0.2.1:1234alive42#labels975109414
}

func ExampleMember_checksumString_multilabels() {
	var b bytes.Buffer
	m := Member{
		Address:     "192.0.2.1:1234",
		Status:      Alive,
		Incarnation: 42,
		Labels: LabelMap{
			"hello": "world",
			"foo":   "baz",
		},
	}
	m.checksumString(&b)
	fmt.Println(b.String())
	// Output: 192.0.2.1:1234alive42#labels-1625122257
}
