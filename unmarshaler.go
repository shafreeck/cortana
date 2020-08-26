package cortana

import "encoding/json"

// JSONUnmarshaler is a json unmarshaler
type JSONUnmarshaler struct{}

// Unmarshal the json data
func (u *JSONUnmarshaler) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
