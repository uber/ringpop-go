package typed

// Copyright (c) 2015 Uber Technologies, Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimple(t *testing.T) {
	buf := make([]byte, 200)

	var r ReadBuffer
	var w WriteBuffer

	{
		w.Wrap(buf)
		w.WriteSingleByte(0xFC)
		r.Wrap(buf)
		assert.Equal(t, byte(0xFC), r.ReadSingleByte())
	}

	{
		w.Wrap(buf)
		w.WriteUint16(0xDEAD)
		r.Wrap(buf)
		assert.Equal(t, uint16(0xDEAD), r.ReadUint16())
	}

	{
		w.Wrap(buf)
		w.WriteUint32(0xBEEFDEAD)
		r.Wrap(buf)
		assert.Equal(t, uint32(0xBEEFDEAD), r.ReadUint32())
	}
}

func TestShortBuffer(t *testing.T) {
	r := NewReadBuffer([]byte{23})
	assert.EqualValues(t, 0, r.ReadUint16())
	assert.Equal(t, ErrEOF, r.Err())
}

func TestReadWrite(t *testing.T) {
	s := "the small brown fix"
	bslice := []byte("jumped over the lazy dog")

	w := NewWriteBufferWithSize(1024)
	w.WriteUint64(0x0123456789ABCDEF)
	w.WriteUint32(0xABCDEF01)
	w.WriteUint16(0x2345)
	w.WriteSingleByte(0xFF)
	w.WriteString(s)
	w.WriteBytes(bslice)
	w.WriteLen8String("hello")
	w.WriteLen16String("This is a much larger string")
	require.NoError(t, w.Err())

	var b bytes.Buffer
	w.FlushTo(&b)

	r := NewReadBufferWithSize(1024)
	r.FillFrom(bytes.NewReader(b.Bytes()), len(b.Bytes()))

	assert.Equal(t, uint64(0x0123456789ABCDEF), r.ReadUint64())
	assert.Equal(t, uint32(0xABCDEF01), r.ReadUint32())
	assert.Equal(t, uint16(0x2345), r.ReadUint16())
	assert.Equal(t, byte(0xFF), r.ReadSingleByte())
	assert.Equal(t, s, r.ReadString(len(s)))
	assert.Equal(t, bslice, r.ReadBytes(len(bslice)))
	assert.Equal(t, "hello", r.ReadLen8String())
	assert.Equal(t, "This is a much larger string", r.ReadLen16String())

	require.NoError(t, r.Err())
}

func TestDeferredWrites(t *testing.T) {
	w := NewWriteBufferWithSize(1024)
	u16ref := w.DeferUint16()
	require.NotNil(t, u16ref)

	u32ref := w.DeferUint32()
	require.NotNil(t, u32ref)

	u64ref := w.DeferUint64()
	require.NotNil(t, u64ref)

	bref := w.DeferBytes(5)
	require.NotNil(t, bref)

	sref := w.DeferBytes(5)
	require.NotNil(t, sref)

	byteref := w.DeferByte()
	require.NotNil(t, byteref)

	assert.Equal(t, 2+4+8+5+5+1, w.BytesWritten())

	u16ref.Update(2040)
	u32ref.Update(495404)
	u64ref.Update(0x40950459)
	bref.Update([]byte{0x30, 0x12, 0x45, 0x55, 0x65})
	sref.UpdateString("where")
	byteref.Update(0x44)

	var buf bytes.Buffer
	w.FlushTo(&buf)

	r := NewReadBuffer(buf.Bytes())

	u16 := r.ReadUint16()
	assert.Equal(t, uint16(2040), u16)

	u32 := r.ReadUint32()
	assert.Equal(t, uint32(495404), u32)

	u64 := r.ReadUint64()
	assert.Equal(t, uint64(0x40950459), u64)

	b := r.ReadBytes(5)
	assert.Equal(t, []byte{0x30, 0x12, 0x45, 0x55, 0x65}, b)

	s := r.ReadString(5)
	assert.Equal(t, "where", s)

	u8 := r.ReadSingleByte()
	assert.Equal(t, byte(0x44), u8)
	assert.NoError(t, r.Err())
}
