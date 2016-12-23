package hashring

type fakeMember struct {
	address  string
	identity string
}

func (f fakeMember) GetAddress() string {
	return f.address
}

func (f fakeMember) Label(key string) (value string, has bool) {
	return "", false
}

func (f fakeMember) Identity() string {
	if f.identity != "" {
		return f.identity
	}
	return f.address
}
