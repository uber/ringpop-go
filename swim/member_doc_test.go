package swim

import (
	"bytes"
	"fmt"
)

func ExampleMember_checksumString_output() {
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
