package cortana

// Unmarshaler unmarshals data to v
type Unmarshaler interface {
	Unmarshal(data []byte, v interface{}) error
}

// UnmarshalFunc turns a func to Unmarshaler
type UnmarshalFunc func(data []byte, v interface{}) error

// Unmarshal the data
func (f UnmarshalFunc) Unmarshal(data []byte, v interface{}) error {
	return f(data, v)
}
