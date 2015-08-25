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

func nonLocalOverride(member *Member, change Change) bool {
	return aliveOverride(member, change) || suspectOverride(member, change) ||
		faultyOverride(member, change) || leaveOverride(member, change)
}

func aliveOverride(member *Member, change Change) bool {
	return change.Status == Alive && change.Incarnation > member.Incarnation
}

func faultyOverride(member *Member, change Change) bool {
	return change.Status == Faulty &&
		((member.Status == Suspect && change.Incarnation >= member.Incarnation) ||
			(member.Status == Faulty && change.Incarnation > member.Incarnation) ||
			(member.Status == Alive && change.Incarnation >= member.Incarnation))
}

func leaveOverride(member *Member, change Change) bool {
	return change.Status == Leave &&
		member.Status != Leave &&
		change.Incarnation >= member.Incarnation
}

func suspectOverride(member *Member, change Change) bool {
	return change.Status == Suspect &&
		((member.Status == Suspect && change.Incarnation > member.Incarnation) ||
			(member.Status == Faulty && change.Incarnation > member.Incarnation) ||
			(member.Status == Alive && change.Incarnation >= member.Incarnation))
}

func localOverride(local string, member *Member, change Change) bool {
	return localSuspectOverride(local, member, change) || localFaultyOverride(local, member, change)
}

func localFaultyOverride(local string, member *Member, change Change) bool {
	return member.Address == local && change.Status == Faulty
}

func localSuspectOverride(local string, member *Member, change Change) bool {
	return member.Address == local && change.Status == Suspect
}
