package cortana

type longshort struct {
	long  string
	short string
}

type config struct {
	path        string
	unmarshaler Unmarshaler
}
