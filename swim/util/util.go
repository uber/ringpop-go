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

package util

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// HostportPattern is regex to match a host:port
var HostportPattern = regexp.MustCompile(`^(\d+.\d+.\d+.\d+):\d+$`)

// CaptureHost takes a host:port and returns the host.
func CaptureHost(hostport string) string {
	if HostportPattern.Match([]byte(hostport)) {
		parts := strings.Split(hostport, ":")
		return parts[0]
	}
	return ""
}

// ReadHostsFile reads a file containing a JSON array of hosts
func ReadHostsFile(file string) ([]string, error) {
	var hosts []string

	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &hosts)
	if err != nil {
		return nil, err
	}

	return hosts, nil
}

// CheckHostnameIPMismatch checks for a hostname/IP mismatch in a map of hosts to
// host:ports. If there is a mismatch, returns the mismatched hosts and an error,
// otherwise nil and nil.
func CheckHostnameIPMismatch(local string, hostsMap map[string][]string) ([]string, error) {
	var message string

	test := func(message string, filter func(string) bool) ([]string, error) {
		var mismatched []string

		for _, hostports := range hostsMap {
			for _, hostport := range hostports {
				if filter(hostport) {
					mismatched = append(mismatched, hostport)
				}
			}
		}

		if len(mismatched) > 0 {
			return mismatched, errors.New(message)
		}

		return nil, nil
	}

	if HostportPattern.MatchString(local) {
		message = "Your host identifier looks like an IP address and there are bootstrap " +
			"hosts that appear to be specified with hostnames. These inconsistencies may " +
			"lead to subtle node communication issues."
		return test(message, func(hostport string) bool {
			return !HostportPattern.MatchString(hostport)
		})
	}

	message = "Your host identifier looks like a hostname and there are bootstrap hosts " +
		"that appear to be specified with IP addresses. These inconsistencies may lead " +
		"to subtle node communication issues"

	return test(message, func(hostport string) bool {
		return HostportPattern.MatchString(hostport)
	})
}

// CheckLocalMissing checks a slice of host:ports for the given local host:port, return
// an error if not found, otherwise nil.
func CheckLocalMissing(local string, hostports []string) error {
	if IndexOf(hostports, local) == -1 {
		return errors.New("local node missing from hosts")
	}

	return nil
}

// SingleNodeCluster determines if local is the only host:port contained within
// the hostsMap
func SingleNodeCluster(local string, hostsMap map[string][]string) bool {
	_, ok := hostsMap[CaptureHost(local)]

	var n int
	for _, hostports := range hostsMap {
		n += len(hostports)
	}
	return ok && n == 1
}

// TimeZero returns a time such that time.IsZero() is true
func TimeZero() time.Time {
	return time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
}

// TimeNowMS returns Unix time in milliseconds for time.Now()
func TimeNowMS() int64 {
	return UnixMS(time.Now())
}

// MS returns the number of milliseconds given in a duration
func MS(d time.Duration) int64 {
	return d.Nanoseconds() / 1000000
}

// UnixMS returns Unix time in milliseconds for the given time
func UnixMS(t time.Time) int64 {
	return t.UnixNano() / 1000000
}

// IndexOf returns the index of element in slice, or -1 if the element is not in slice
func IndexOf(slice []string, element string) int {
	for i, e := range slice {
		if e == element {
			return i
		}
	}

	return -1
}

// TakeNode takes an element from nodes at the given index, or at a random index if
// index < 0. Mutates nodes.
func TakeNode(nodes *[]string, index int) string {
	if len(*nodes) == 0 {
		return ""
	}

	var i int
	if index >= 0 {
		if index >= len(*nodes) {
			return ""
		}
		i = index
	} else {
		i = rand.Intn(len(*nodes))
	}

	node := (*nodes)[i]

	*nodes = append((*nodes)[:i], (*nodes)[i+1:]...)

	return node
}

// SelectInt takes an option and a default value and returns the default value if
// the option is equal to zero, and the option otherwise.
func SelectInt(opt, def int) int {
	if opt == 0 {
		return def
	}
	return opt
}

// SelectDuration takes an option and a default value and returns the default value if
// the option is equal to zero, and the option otherwise.
func SelectDuration(opt, def time.Duration) time.Duration {
	if opt == time.Duration(0) {
		return def
	}
	return opt
}

// Min returns min(a,b)
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Bi-directional binding of Time to interger Unix timestamp in JSON
type Timestamp time.Time

func (t *Timestamp) MarshalJSON() ([]byte, error) {
	ts := time.Time(*t).Unix()
	stamp := fmt.Sprint(ts)

	return []byte(stamp), nil
}

func (t *Timestamp) UnmarshalJSON(b []byte) error {
	ts, err := strconv.Atoi(string(b))
	if err != nil {
		return err
	}

	*t = Timestamp(time.Unix(int64(ts), 0))

	return nil
}
