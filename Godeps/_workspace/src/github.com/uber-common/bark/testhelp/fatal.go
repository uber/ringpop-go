// Copyright (c) 2015 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package main

import
(
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/uber/bark"
)

func main() {
	var logrusLogger *logrus.Logger = logrus.New()
	logrusLogger.Formatter = new(logrus.JSONFormatter)

	if len(os.Args) != 2 {
		logrus.Error("Must pass an arg to test program...")
		os.Exit(0)
	}

	switch os.Args[1] {
	case "logrus.Fatal":
		logrusLogger.Fatal("fatal error")
	case "logrus.Fatalf":
		logrusLogger.Fatalf("fatal error%s", "fatal error")
	case "bark.Fatal":
		bark.NewLoggerFromLogrus(logrusLogger).Fatal("fatal error")
	case "bark.Fatalf":
		bark.NewLoggerFromLogrus(logrusLogger).Fatalf("fatal error%s", "fatal error")
	}

	logrus.Error("Expected fatal methods to exit...")
	os.Exit(0)
}
