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

// EnvUnmarshaler unmarshals the environment variables
type EnvUnmarshaler interface {
	Unmarshal(v interface{}) error
}

// EnvUnmarshalFunc turns a func to an EnvUnmarshaler
type EnvUnmarshalFunc func(v interface{}) error

// Unmarshal the environment variables
func (f EnvUnmarshalFunc) Unmarshal(v interface{}) error {
	return f(v)
}
