package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/cactus/go-statsd-client/statsd"
)

// fileSender is an adapter from io.WriteCloser to statsd.Sender
type fileSender struct {
	bufWriter *bufio.Writer
	closer    io.Closer
}

// Send implements the statsd.Sender interface
func (fs *fileSender) Send(data []byte) (int, error) {
	line := fmt.Sprintf("%s: %s\n", time.Now().UTC().Format(time.RFC3339Nano), data)
	_, err := fs.bufWriter.Write([]byte(line))
	// Because we're changing the underlying bytes sent, make sure:
	//   written == len(data) on success
	//   written < len(data) on error
	if err != nil {
		return 0, err
	}
	return len(data), nil
}

// Close implements the statsd.Sender interface
func (fs *fileSender) Close() error {
	err := fs.bufWriter.Flush()
	fs.closer.Close()
	return err
}

// NewWriteCloserToSender returns a new WriteCloserToSender adapter
func newFileSender(wc io.WriteCloser) *fileSender {
	return &fileSender{
		bufWriter: bufio.NewWriter(wc),
		closer:    wc}
}

// NewFileStatsd returns a statsd.Statter that writes to file. Each entry is
// prefixed by a date/time with nanosecond resolution on it's own line
func NewFileStatsd(name string) (statsd.Statter, error) {
	f, err := os.Create(name)
	if err != nil {
		return nil, err
	}
	fileSender := newFileSender(f)
	c, err := statsd.NewClientWithSender(fileSender, "")
	if err != nil {
		return nil, err
	}
	return c, nil
}
