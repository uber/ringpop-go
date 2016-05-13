package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/cactus/go-statsd-client/statsd"
)

// WriteCloserToSender is an adapter from io.WriteCloser to statsd.Sender
type WriteCloserToSender struct {
	bufWriter *bufio.Writer
	closer    io.Closer
}

// Send implements the statsd.Sender interface
func (w *WriteCloserToSender) Send(data []byte) (int, error) {
	return w.bufWriter.Write(data)
}

// Close implements the statsd.Sender interface
func (w *WriteCloserToSender) Close() error {
	err := w.bufWriter.Flush()
	w.closer.Close()
	return err
}

// NewWriteCloserToSender returns a new WriteCloserToSender adapter
func NewWriteCloserToSender(wc io.WriteCloser) statsd.Sender {
	return &WriteCloserToSender{
		bufWriter: bufio.NewWriter(wc),
		closer:    wc}
}

// FormattedSender wraps a statsd.Sender and adds a timestamp and new lines
// after each Send call.
type FormattedSender struct {
	s statsd.Sender
}

// Send implements the statsd.Sender interface
func (fs *FormattedSender) Send(data []byte) (int, error) {
	line := fmt.Sprintf("%s: %s\n", time.Now().Format(time.StampMicro), data)
	_, err := fs.s.Send([]byte(line))
	// Because we're changing the undelying bytes sent, make sure:
	//   written == len(data) on success
	//   written < len(data) on error
	if err != nil {
		return 0, err
	}
	return len(data), nil
}

// Close implements the stats.Sender interface
func (fs *FormattedSender) Close() error {
	return fs.s.Close()
}

// NewFileStatsd returns a statsd.Statter that writes to file. Each entry is
// prefixed by a date/time with nanosecond resolution on it's own line
func NewFileStatsd(name string) (statsd.Statter, error) {
	f, err := os.Create(name)
	if err != nil {
		return nil, err
	}
	s := &FormattedSender{NewWriteCloserToSender(f)}
	c, err := statsd.NewClientWithSender(s, "")
	if err != nil {
		return nil, err
	}
	return c, nil
}
