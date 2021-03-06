package cortana

import (
	"reflect"
	"strings"
)

type flag struct {
	name         string // the field name
	long         string
	short        string
	required     bool
	defaultValue string
	description  string
	rv           reflect.Value
}

// nonflag is in fact a flag without prefix "-"
type nonflag flag

func parseFlag(tag string, name string, rv reflect.Value) *flag {
	f := &flag{name: name, rv: rv}
	parts := strings.Split(tag, ",")

	const (
		long = iota
		short
		defaultValue
		description
	)
	state := long
	for i := 0; i < len(parts); i++ {
		p := strings.TrimSpace(parts[i])
		switch state {
		case long:
			f.long = p
			state = short
		case short:
			f.short = p
			state = defaultValue
		case defaultValue:
			if p == "-" {
				f.required = true
			} else {
				// set to empty value
				if p == `''` || p == `""` {
					p = ""
				}
				f.defaultValue = p
			}
			state = description
		case description:
			f.description = strings.TrimSpace(strings.Join(parts[i:], ","))
			return f
		}
	}
	return f
}
