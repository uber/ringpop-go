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
	"errors"
	"fmt"
	"math/rand"
	"net"
	"regexp"
	"strconv"
	"time"
)

// HostportPattern is regex to match a host:port
var HostportPattern = regexp.MustCompile(`^(\d+.\d+.\d+.\d+):\d+$`)

// CaptureHost takes a host:port and returns the host.
func CaptureHost(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return ""
	}
	return host
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
	if !StringInSlice(hostports, local) {
		return errors.New("local node missing from hosts")
	}

	return nil
}

// SingleNodeCluster determines if hostport is the only host:port contained within
// the hostMap.
func SingleNodeCluster(hostport string, hostMap map[string][]string) bool {
	// If the map contains more than one host then clearly there is more than
	// one address so we can't be a single node cluster.
	if len(hostMap) > 1 {
		return false
	}

	host := CaptureHost(hostport)

	// If hostport is not even in the map at all then it's not possible for
	// us to be a single node cluster.
	_, ok := hostMap[host]
	if !ok {
		return false
	}

	// Same as above; there is most than one host
	if len(hostMap[host]) > 1 {
		return false
	}

	// There is a single hostport in the map; check that it matches
	if hostMap[host][0] == hostport {
		return true
	}

	// There is a single hostport in the map but it doesn't match the given
	// hostport.
	return false
}

// HostPortsByHost parses a list of host/port conbinations and creates a map
// of unique hosts each containing a slice of the hostsport instances for each
// host.
func HostPortsByHost(hostports []string) (hostMap map[string][]string) {
	hostMap = make(map[string][]string)
	for _, hostport := range hostports {
		host := CaptureHost(hostport)
		if host != "" {
			hostMap[host] = append(hostMap[host], hostport)
		}
	}
	return
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

// StringInSlice returns whether the string is contained within the slice.
func StringInSlice(slice []string, str string) bool {
	for _, b := range slice {
		if str == b {
			return true
		}
	}
	return false
}

// ShuffleStrings takes a slice of strings and returns a new slice containing
// the same strings in a random order.
func ShuffleStrings(strings []string) []string {
	newStrings := make([]string, len(strings))
	newIndexes := rand.Perm(len(strings))

	for o, n := range newIndexes {
		newStrings[n] = strings[o]
	}

	return newStrings
}

// ShuffleStringsInPlace uses the Fisher–Yates shuffle to randomize the strings
// in place.
func ShuffleStringsInPlace(strings []string) {
	for i := range strings {
		j := rand.Intn(i + 1)
		strings[i], strings[j] = strings[j], strings[i]
	}
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

// SelectFloat takes an option and a default value and returns the default value if
// the option is equal to zero, and the option otherwise.
func SelectFloat(opt, def float64) float64 {
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

// SelectBool takes an option and a default value and returns the default value if
// the option is equal to the zero value, and the option otherwise.
func SelectBool(opt, def bool) bool {
	if opt == false {
		return def
	}
	return opt
}

// Min returns the lowest integer and is defined because golang only has a min
// function for floats and not for ints.
func Min(first int, rest ...int) int {
	m := first
	for _, value := range rest {
		if value < m {
			m = value
		}
	}
	return m
}

// Timestamp is a bi-directional binding of Time to interger Unix timestamp in
// JSON.
type Timestamp time.Time

// MarshalJSON returns the JSON encoding of the timestamp.
func (t *Timestamp) MarshalJSON() ([]byte, error) {
	ts := time.Time(*t).Unix()
	stamp := fmt.Sprint(ts)

	return []byte(stamp), nil
}

// UnmarshalJSON sets the timestamp to the value in the specified JSON.
func (t *Timestamp) UnmarshalJSON(b []byte) error {
	ts, err := strconv.Atoi(string(b))
	if err != nil {
		return err
	}

	*t = Timestamp(time.Unix(int64(ts), 0))

	return nil
}
