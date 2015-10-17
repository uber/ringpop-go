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

// Package trace provides methods to submit Zipkin style Span to tcollector Server.
package trace

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	tc "github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/thrift"
	"github.com/uber/tchannel-go/trace/thrift/gen-go/tcollector"
)

const (
	tcollectorServiceName = "tcollector"
	chanBufferSize        = 100
)

type zipkinData struct {
	Span              tc.Span
	Annotations       []tc.Annotation
	BinaryAnnotations []tc.BinaryAnnotation
	TargetEndpoint    tc.TargetEndpoint
}

// ZipkinTraceReporter is a trace reporter that submits trace spans in to zipkin trace server.
type ZipkinTraceReporter struct {
	tchannel *tc.Channel
	client   tcollector.TChanTCollector
	c        chan zipkinData
	logger   tc.Logger
}

// NewZipkinTraceReporter returns a zipkin trace reporter that submits span to tcollector service.
func NewZipkinTraceReporter(ch *tc.Channel) *ZipkinTraceReporter {
	thriftClient := thrift.NewClient(ch, tcollectorServiceName, nil)
	client := tcollector.NewTChanTCollectorClient(thriftClient)
	// create the goroutine method to actually to the submit Span.
	reporter := &ZipkinTraceReporter{
		tchannel: ch,
		client:   client,
		c:        make(chan zipkinData, chanBufferSize),
		logger:   ch.Logger(),
	}
	go reporter.zipkinSpanWorker()
	return reporter
}

// Report method will submit trace span to tcollector server.
func (r *ZipkinTraceReporter) Report(
	span tc.Span, annotations []tc.Annotation, binaryAnnotations []tc.BinaryAnnotation, targetEndpoint tc.TargetEndpoint) {
	data := zipkinData{
		Span:              span,
		Annotations:       annotations,
		BinaryAnnotations: binaryAnnotations,
		TargetEndpoint:    targetEndpoint,
	}

	select {
	case r.c <- data:
	default:
		r.logger.Infof("Buffer channel for zipkin trace report is full.")
	}
}

func (r *ZipkinTraceReporter) zipkinReport(data *zipkinData) error {
	ctx, cancel := tc.NewContextBuilder(time.Second).
		DisableTracing().
		SetShardKey(base64Encode(data.Span.TraceID())).Build()
	defer cancel()

	thriftSpan, err := buildZipkinSpan(data.Span, data.Annotations, data.BinaryAnnotations, data.TargetEndpoint)
	if err != nil {
		return err
	}
	// client submit
	// ignore the response result because TChannel shouldn't care about it.
	_, err = r.client.Submit(ctx, thriftSpan)
	return err
}

func (r *ZipkinTraceReporter) zipkinSpanWorker() {
	for data := range r.c {
		if err := r.zipkinReport(&data); err != nil {
			r.logger.Infof("Zipkin Span submit failed. Get error: %v", err)
		}
	}
}

// buildZipkinSpan builds zipkin span based on tchannel span.
func buildZipkinSpan(span tc.Span, annotations []tc.Annotation, binaryAnnotations []tc.BinaryAnnotation, targetEndpoint tc.TargetEndpoint) (*tcollector.Span, error) {
	hostport := strings.Split(targetEndpoint.HostPort, ":")
	port, _ := strconv.ParseInt(hostport[1], 10, 32)
	host := tcollector.Endpoint{
		Ipv4:        int32(inetAton(hostport[0])),
		Port:        int32(port),
		ServiceName: targetEndpoint.ServiceName,
	}

	tBinaryAnnotations, err := buildBinaryAnnotations(binaryAnnotations)
	if err != nil {
		return nil, err
	}
	thriftSpan := tcollector.Span{
		TraceId:           uint64ToBytes(span.TraceID()),
		Host:              &host,
		Name:              targetEndpoint.Operation,
		ID:                uint64ToBytes(span.SpanID()),
		ParentId:          uint64ToBytes(span.ParentID()),
		Annotations:       buildZipkinAnnotations(annotations),
		BinaryAnnotations: tBinaryAnnotations,
	}

	return &thriftSpan, nil
}

func buildBinaryAnnotation(ann tc.BinaryAnnotation) (*tcollector.BinaryAnnotation, error) {
	bann := &tcollector.BinaryAnnotation{Key: ann.Key}
	switch v := ann.Value.(type) {
	case bool:
		bann.AnnotationType = tcollector.AnnotationType_BOOL
		bann.BoolValue = &v
	case int64:
		bann.AnnotationType = tcollector.AnnotationType_I64
		temp := v
		bann.IntValue = &temp
	case int32:
		bann.AnnotationType = tcollector.AnnotationType_I32
		temp := int64(v)
		bann.IntValue = &temp
	case int16:
		bann.AnnotationType = tcollector.AnnotationType_I16
		temp := int64(v)
		bann.IntValue = &temp
	case int:
		bann.AnnotationType = tcollector.AnnotationType_I32
		temp := int64(v)
		bann.IntValue = &temp
	case string:
		bann.AnnotationType = tcollector.AnnotationType_STRING
		bann.StringValue = &v
	case []byte:
		bann.AnnotationType = tcollector.AnnotationType_BYTES
		bann.BytesValue = v
	case float32:
		bann.AnnotationType = tcollector.AnnotationType_DOUBLE
		temp := float64(v)
		bann.DoubleValue = &temp
	case float64:
		bann.AnnotationType = tcollector.AnnotationType_DOUBLE
		temp := float64(v)
		bann.DoubleValue = &temp
	default:
		return nil, fmt.Errorf("unrecognized data type: %T", v)
	}
	return bann, nil
}

func buildBinaryAnnotations(anns []tc.BinaryAnnotation) ([]*tcollector.BinaryAnnotation, error) {
	binaryAnns := make([]*tcollector.BinaryAnnotation, len(anns))
	for i, ann := range anns {
		b, err := buildBinaryAnnotation(ann)
		binaryAnns[i] = b
		if err != nil {
			return nil, err
		}
	}
	return binaryAnns, nil
}

// buildZipkinAnnotations builds zipkin Annotations based on tchannel annotations.
func buildZipkinAnnotations(anns []tc.Annotation) []*tcollector.Annotation {
	zipkinAnns := make([]*tcollector.Annotation, len(anns))
	for i, ann := range anns {
		zipkinAnns[i] = &tcollector.Annotation{
			Timestamp: float64(ann.Timestamp.UnixNano() / 1e6),
			Value:     string(ann.Key),
		}
	}
	return zipkinAnns
}

// inetAton converts string Ipv4 to uint32
func inetAton(ip string) uint32 {
	ipBytes := net.ParseIP(ip).To4()
	return binary.BigEndian.Uint32(ipBytes)
}

// base64Encode encodes uint64 with base64 StdEncoding.
func base64Encode(data uint64) string {
	return base64.StdEncoding.EncodeToString(uint64ToBytes(data))
}

// uint64ToBytes converts uint64 to bytes.
func uint64ToBytes(i uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(i))
	return buf
}

// ZipkinTraceReporterFactory builds ZipkinTraceReporter by given TChannel instance.
func ZipkinTraceReporterFactory(tchannel *tc.Channel) tc.TraceReporter {
	return NewZipkinTraceReporter(tchannel)
}
