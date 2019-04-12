package bytefmt_test

import (
	"testing"

	. "code.cloudfoundry.org/bytefmt"
)

const (
	byteSize = 1e10
	toBytes  = "\n\n\r\t10.18TiB\n\n\r\t"
)

func BenchmarkToBytes(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := ToBytes(toBytes); err != nil {
			b.Errorf("error: %s", err)
		}
	}
}

func BenchmarkByteSize(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ByteSize(byteSize)
	}
}
