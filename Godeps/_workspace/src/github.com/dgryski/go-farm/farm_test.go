package farm

import "testing"

// Generated from the C++ code
var golden32 = []struct {
	out uint32
	in  string
}{
	{0x3c973d4d, "a"},
	{0x417330fd, "ab"},
	{0x2f635ec7, "abc"},
	{0x98b51e95, "abcd"},
	{0xa3f366ac, "abcde"},
	{0x0f813aa4, "abcdef"},
	{0x21deb6d7, "abcdefg"},
	{0xfd7ec8b9, "abcdefgh"},
	{0x6f98dc86, "abcdefghi"},
	{0xf2669361, "abcdefghij"},
	{0xe273108f, "Discard medicine more than two years old."},
	{0xf585dfc4, "He who has a shady past knows that nice guys finish last."},
	{0x363394d1, "I wouldn't marry him with a ten foot pole."},
	{0x7613810f, "Free! Free!/A trip/to Mars/for 900/empty jars/Burma Shave"},
	{0x2cc30bb7, "The days of the digital watch are numbered.  -Tom Stoppard"},
	{0x322984d9, "Nepal premier won't resign."},
	{0xa5812ac8, "For every action there is an equal and opposite government program."},
	{0x1090d244, "His money is twice tainted: 'taint yours and 'taint mine."},
	{0xff16c9e6, "There is no reason for any individual to have a computer in their home. -Ken Olsen, 1977"},
	{0xcc3d0ff2, "It's a tiny change to the code and not completely disgusting. - Bob Manchek"},
	{0xc6246b8d, "size:  a.out:  bad magic"},
	{0xd225e92e, "The major problem is with sendmail.  -Mark Horton"},
	{0x1b8db5d0, "Give me a rock, paper and scissors and I will move the world.  CCFestoon"},
	{0x4fda5f07, "If the enemy is within range, then so are you."},
	{0x2e18e880, "It's well we cannot hear the screams/That we create in others' dreams."},
	{0xd07de88f, "You remind me of a TV show, but that's all right: I watch it anyway."},
	{0x221694e4, "C is as portable as Stonehedge!!"},
	{0xe2053c2c, "Even if I could be Shakespeare, I think I should still choose to be Faraday. - A. Huxley"},
	{0x11c493bb, "The fugacity of a constituent in a mixture of gases at a given temperature is proportional to its mole fraction.  Lewis-Randall Rule"},
	{0x0819a4e8, "How can you write a big system without C++?  -Paul Glick"},
}

func TestHash32(t *testing.T) {

	for _, tt := range golden32 {
		if h := Hash32([]byte(tt.in)); h != tt.out {
			t.Errorf("Hash32(%q)=%#08x (len=%d), want %#08x", tt.in, h, len(tt.in), tt.out)
		}
	}

}

// Generated from the C++ code
var golden64 = []struct {
	out uint64
	in  string
}{
	{0xb3454265b6df75e3, "a"},
	{0xaa8d6e5242ada51e, "ab"},
	{0x24a5b3a074e7f369, "abc"},
	{0x1a5502de4a1f8101, "abcd"},
	{0xc22f4663e54e04d4, "abcde"},
	{0xc329379e6a03c2cd, "abcdef"},
	{0x3c40c92b1ccb7355, "abcdefg"},
	{0xfee9d22990c82909, "abcdefgh"},
	{0x332c8ed4dae5ba42, "abcdefghi"},
	{0x8a3abb6a5f3fb7fb, "abcdefghij"},
	{0xe8f89ab6df9bdd25, "Discard medicine more than two years old."},
	{0x786d7e1987023ca9, "He who has a shady past knows that nice guys finish last."},
	{0xa9961670ce2a46d9, "I wouldn't marry him with a ten foot pole."},
	{0x5d14f96c18fe3d5e, "Free! Free!/A trip/to Mars/for 900/empty jars/Burma Shave"},
	{0x2a578b80bb82147c, "The days of the digital watch are numbered.  -Tom Stoppard"},
	{0x8eb3808d1ccfc779, "Nepal premier won't resign."},
	{0x55182f8859eca4ce, "For every action there is an equal and opposite government program."},
	{0xec8848fd3b266c10, "His money is twice tainted: 'taint yours and 'taint mine."},
	{0x7e85d7b050ed2967, "There is no reason for any individual to have a computer in their home. -Ken Olsen, 1977"},
	{0x38aa3175b37f305c, "It's a tiny change to the code and not completely disgusting. - Bob Manchek"},
	{0x80d73b843ba57db8, "size:  a.out:  bad magic"},
	{0xc2f8db8624fefc0e, "The major problem is with sendmail.  -Mark Horton"},
	{0xa2b8bf3032021993, "Give me a rock, paper and scissors and I will move the world.  CCFestoon"},
	{0xbdd69b798d6ba37a, "If the enemy is within range, then so are you."},
	{0x1d85702503ac7eb4, "It's well we cannot hear the screams/That we create in others' dreams."},
	{0xabcdb319fcf2826c, "You remind me of a TV show, but that's all right: I watch it anyway."},
	{0xb944f8a16261e414, "C is as portable as Stonehedge!!"},
	{0x5a05644eb66e435e, "Even if I could be Shakespeare, I think I should still choose to be Faraday. - A. Huxley"},
	{0x98eff6958c5e91a, "The fugacity of a constituent in a mixture of gases at a given temperature is proportional to its mole fraction.  Lewis-Randall Rule"},
	{0x5a0a6efd52e84e2a, "How can you write a big system without C++?  -Paul Glick"},
}

func TestHash64(t *testing.T) {
	for _, tt := range golden64 {
		if h := Hash64([]byte(tt.in)); h != tt.out {
			t.Errorf("Hash64(%q)=%#016x, (len=%d) want %#016x", tt.in, h, len(tt.in), tt.out)
		}

	}
}

func TestFingerprint128(t *testing.T) {

	var tests = []struct {
		hi, lo uint64
		in     string
	}{
		{9054869399155703984, 8033370924408288235, "abcdef"},
		{352412539875473798, 3547689611939963773, "There is no reason for any individual to have a computer in their home. -Ken Olsen, 1977"},
		{14320160249354795919, 10805939018293574989, "The fugacity of a constituent in a mixture of gases at a given temperature is proportional to its mole fraction.  Lewis-Randall RuleAAAAAAAAAAAAAAAA"},
	}

	for _, tt := range tests {
		if lo, hi := Fingerprint128([]byte(tt.in)); hi != tt.hi || lo != tt.lo {
			t.Errorf("Fingerprint128(%q)=(%#016x, %#016x) (len=%d) want (%#016x, %#016x)", tt.in, lo, hi, len(tt.in), tt.lo, tt.hi)
		}
	}
}
