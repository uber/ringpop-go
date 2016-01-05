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

package swim

import (
	"encoding/json"
	"errors"
	"io/ioutil"
)

// A DiscoverProvider is a interface that provides a list of peers for a node
// to bootstrap from.
type DiscoverProvider interface {
	Hosts() ([]string, error)
}

// JSONFileHostList is a DiscoverProvider that reads a list of hosts from a
// JSON file.
type JSONFileHostList struct {
	filePath string
}

// Hosts reads hosts from a JSON file.
func (p *JSONFileHostList) Hosts() ([]string, error) {
	var hosts []string

	data, err := ioutil.ReadFile(p.filePath)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &hosts)
	if err != nil {
		return nil, err
	}

	return hosts, nil
}

// StaticHostList is a static list of hosts to bootstrap from.
type StaticHostList struct {
	hosts []string
}

// Hosts just returns the static list of hosts that was provided to this struct
// on construction.
func (p *StaticHostList) Hosts() ([]string, error) {
	return p.hosts, nil
}

// resolveDiscoverProvider reads legacy BootstrapOptions and determines which
// DiscoverProvider implementation to use for enumerating a list of bootstrap
// hosts.
// TODO: Remove this function and legacy File/Hosts fields from
// BootstrapOptions and accept only a DiscoverProvider.
func resolveDiscoverProvider(opts *BootstrapOptions) (DiscoverProvider, error) {
	if opts.DiscoverProvider != nil {
		return opts.DiscoverProvider, nil
	}
	if len(opts.Hosts) != 0 {
		return &StaticHostList{opts.Hosts}, nil
	}
	if len(opts.File) != 0 {
		return &JSONFileHostList{opts.File}, nil
	}
	return nil, errors.New("no discover provider")
}
