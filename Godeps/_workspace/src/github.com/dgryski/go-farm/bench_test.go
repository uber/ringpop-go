package farm

import "testing"

func BenchmarkShort(b *testing.B) {

	buf := make([]byte, 32)

	for i := 0; i < b.N; i++ {
		Hash32(buf)
	}
}

func BenchmarkHash64(b *testing.B) {

	buf := make([]byte, 2048)

	for i := 0; i < b.N; i++ {
		Hash64(buf)
	}
}
