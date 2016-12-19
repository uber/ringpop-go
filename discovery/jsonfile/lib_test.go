// Copyright (c) 2016 Uber Technologies, Inc.
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

package jsonfile

import (
	"io/ioutil"
	"os"

	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInvalidJSONFile(t *testing.T) {
	provider := New("./invalid")
	_, err := provider.Hosts()
	assert.Error(t, err, "open /invalid: no such file or directory", "should fail to open file")
}

func TestMalformedJSONFile(t *testing.T) {
	provider := New("../test/invalidhosts.json")
	_, err := provider.Hosts()
	assert.Error(t, err, "invalid character 'T' looking for beginning of value", "should fail to unmarhsal JSON")
}

func TestNiceJSONFile(t *testing.T) {
	content := []byte("[\"127.0.0.1:3000\", \"127.0.0.1:3042\"]")

	tmpfile, err := ioutil.TempFile("", "jsonfiletest")
	assert.NoError(t, err, "failed to create a temp file")
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write(content)
	assert.NoError(t, err, "failed to write contents to the temp file")

	err = tmpfile.Close()
	assert.NoError(t, err, "failed to write contents to the temp file")

	// Now actually test the DiscoverProvider:Hosts() will return the two
	provider := New(tmpfile.Name())
	res, err := provider.Hosts()
	assert.NoError(t, err, "hosts call failed")
	assert.Equal(t, []string{"127.0.0.1:3000", "127.0.0.1:3042"}, res)
}
