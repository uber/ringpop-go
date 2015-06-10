package tchannel

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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

type TracingRequest struct{}

type TracingResponse struct {
	TraceID  uint64
	SpanID   uint64
	ParentID uint64
	Child    *TracingResponse
}

type Headers map[string]string

func TestTracingPropagates(t *testing.T) {
	withTestChannel(t, func(ch *Channel, hostPort string) {
		srv1 := func(ctx context.Context, incall *InboundCall) {
			headers := Headers{}

			var request TracingRequest
			if err := incall.ReadArg2(NewJSONInput(&headers)); err != nil {
				return
			}

			if err := incall.ReadArg3(NewJSONInput(&request)); err != nil {
				return
			}

			span := CurrentSpan(ctx)

			var childRequest TracingRequest
			var childResponse TracingResponse

			outcall, err := ch.BeginCall(ctx, hostPort, "TestService", "call2", nil)
			if err != nil {
				incall.Response().SendSystemError(err)
				return
			}

			if err := outcall.WriteArg2(NewJSONOutput(headers)); err != nil {
				incall.Response().SendSystemError(err)
				return
			}

			if err := outcall.WriteArg3(NewJSONOutput(childRequest)); err != nil {
				incall.Response().SendSystemError(err)
				return
			}

			if err := outcall.Response().ReadArg2(NewJSONInput(&headers)); err != nil {
				incall.Response().SendSystemError(err)
				return
			}

			if err := outcall.Response().ReadArg3(NewJSONInput(&childResponse)); err != nil {
				incall.Response().SendSystemError(err)
				return
			}

			response := TracingResponse{
				TraceID: span.TraceID(),
				SpanID:  span.SpanID(),
				Child:   &childResponse,
			}

			incall.Response().WriteArg2(NewJSONOutput(headers))
			incall.Response().WriteArg3(NewJSONOutput(response))
		}

		srv2 := func(ctx context.Context, call *InboundCall) {
			span := CurrentSpan(ctx)
			if span == nil {
				call.Response().SendSystemError(NewSystemError(ErrCodeUnexpected, "tracing not found"))
				return
			}

			call.Response().WriteArg2(NewJSONOutput(Headers{}))
			call.Response().WriteArg3(NewJSONOutput(TracingResponse{
				SpanID:   span.SpanID(),
				TraceID:  span.TraceID(),
				ParentID: span.ParentID(),
			}))
		}

		ch.Register(HandlerFunc(srv1), "TestService", "call1")
		ch.Register(HandlerFunc(srv2), "TestService", "call2")

		ctx, cancel := context.WithTimeout(NewRootContext(context.Background()), 5*time.Second)
		defer cancel()

		headers := map[string]string{}
		var request TracingRequest
		var response TracingResponse

		call, err := ch.BeginCall(ctx, hostPort, "TestService", "call1", nil)
		require.NoError(t, err)
		require.NoError(t, call.WriteArg2(NewJSONOutput(headers)))
		require.NoError(t, call.WriteArg3(NewJSONOutput(&request)))
		require.NoError(t, call.Response().ReadArg2(NewJSONInput(&headers)))
		require.NoError(t, call.Response().ReadArg3(NewJSONInput(&response)))

		clientSpan := CurrentSpan(ctx)
		require.NotNil(t, clientSpan)

		assert.Equal(t, clientSpan.TraceID(), response.TraceID)
		assert.Equal(t, clientSpan.SpanID(), response.ParentID)

		nestedResponse := response.Child
		require.NotNil(t, nestedResponse)
		assert.Equal(t, clientSpan.TraceID(), nestedResponse.TraceID)
		assert.Equal(t, response.SpanID, nestedResponse.ParentID)
	})
}
