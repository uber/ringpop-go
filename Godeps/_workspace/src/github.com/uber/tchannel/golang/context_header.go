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

package tchannel

import "golang.org/x/net/context"

// ContextWithHeaders is a Context which contains request and response headers.
type ContextWithHeaders interface {
	context.Context

	// Headers returns the call request headers.
	Headers() map[string]string

	// ResponseHeaders returns the call response headers.
	ResponseHeaders() map[string]string

	// SetResponseHeaders sets the given response headers on the context.
	SetResponseHeaders(map[string]string)
}

type headerCtx struct {
	context.Context
	reqHeaders  map[string]string
	respHeaders map[string]string
}

// Headers gets application headers out of the context.
func (c *headerCtx) Headers() map[string]string {
	return c.reqHeaders
}

// ResponseHeaders returns the response headers.
func (c *headerCtx) ResponseHeaders() map[string]string {
	return c.respHeaders
}

// SetResponseHeaders sets the response headers.
func (c *headerCtx) SetResponseHeaders(headers map[string]string) {
	c.respHeaders = headers
}

// WrapWithHeaders returns a Context that can be used to make a call with request headers.
func WrapWithHeaders(ctx context.Context, headers map[string]string) ContextWithHeaders {
	return &headerCtx{
		Context:    ctx,
		reqHeaders: headers,
	}
}
