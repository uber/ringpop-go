// Copyright (c) 2015 Uber Technologies, Inc.
//
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

package logger

// Fields redefine bark.Fields for convenience.
type Fields map[string]interface{}

// Fields implements the bark.LogFields interface.
func (f Fields) Fields() map[string]interface{} {
	return f
}

// withField returns a new copy with the extra field set
func (f Fields) withField(key string, value interface{}) Fields {
	newSize := len(f) + 1
	newFields := make(map[string]interface{}, newSize)
	for k, v := range f {
		newFields[k] = v
	}
	newFields[key] = value
	return newFields
}

// withFields returns a new copy updated with other's values
func (f Fields) withFields(other Fields) Fields {
	newSize := len(f) + len(other)
	newFields := make(map[string]interface{}, newSize)
	for k, v := range f {
		newFields[k] = v
	}
	for k, v := range other {
		newFields[k] = v
	}
	return newFields
}
