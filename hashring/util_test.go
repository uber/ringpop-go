package hashring

type fakeMember struct {
	address string
}

func (f fakeMember) GetAddress() string {
	return f.address
}

func (f fakeMember) Label(key string) (value string, has bool) {
	return "", false
}
