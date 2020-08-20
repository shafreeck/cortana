package cortana

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// Command is the unit to run
type Command func()

// desc describes a command
type desc struct {
	title       string
	description string
	flags       string
}

type context struct {
	name string
	args []string
	desc desc
}

// Cortana is the commander
type Cortana struct {
	ctx      *context
	commands map[string]Command
}

// Fatal exit the process with an error
func Fatal(err error) {
	fmt.Println(err)
	os.Exit(-1)
}

// New a Cortana commander
func New() *Cortana {
	return &Cortana{commands: make(map[string]Command)}
}

// AddCommand adds a command
func (c *Cortana) AddCommand(path string, cmd Command) {
	c.commands[path] = cmd
}

// Launch and run commands
func (c *Cortana) Launch() {
	args := os.Args[1:]
	if len(args) == 0 {
		return // TODO usage
	}

	// the arguments with '-' prefix are flags, others are names
	var names []string
	var flags []string
	for i := 0; i < len(args); i++ {
		if args[i][0] == '-' {
			flags = append(flags, args[i])
			if i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
		} else {
			names = append(names, args[i])
		}
	}

	// search for the command
	l := len(names)
	for i := range names {
		path := strings.Join(names[0:l-i], " ")
		cmd, ok := c.commands[path]
		if !ok {
			// no more commands in path
			if i+1 == l {
				Fatal(errors.New("unknow command pattern " + strings.Join(names, " ")))
			}
			continue
		}

		args := append(names[l-i:], flags...)
		c.ctx = &context{
			name: path,
			args: args,
		}
		cmd()
		return
	}
}

// Args returns the args in current context
func (c *Cortana) Args() []string {
	return c.ctx.args
}

// Use the flags
func (c *Cortana) Use(title, description string, v interface{}) {
	flags := c.collectFlags(v)
	c.ctx.desc = desc{
		title:       title,
		description: description,
		flags:       flags,
	}
	UnmarshalArgs(c.ctx.args, v)
}

// Usage prints the usage
func (c *Cortana) Usage() {
	w := bytes.NewBuffer(nil)
	w.WriteString(c.ctx.desc.title + "\n")
	w.WriteString(c.ctx.desc.description + "\n")
	w.WriteString(c.ctx.desc.flags + "\n")
	fmt.Print(w.String())
	os.Exit(0)
}

func (c *Cortana) collectFlags(v interface{}) string {
	flags := make(map[string]*reflect.Value)
	names := buildArgsIndex(flags, reflect.ValueOf(v))

	w := bytes.NewBuffer(nil)
	w.WriteString(c.ctx.name)

	for i := range names {
		rv := names[i]
		name := rv.String()
		// TODO use required flag
		if rv.IsZero() {
			w.WriteString(" <" + name + ">")
		} else {
			w.WriteString(" [" + name + "]")
		}
	}
	w.WriteString("\n")

	for flag := range flags {
		w.WriteString(flag + "\n")
	}
	return w.String()
}

func buildArgsIndex(flags map[string]*reflect.Value, rv reflect.Value) []*reflect.Value {
	names := make([]*reflect.Value, 0)
	for rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		ft := rt.Field(i)
		fv := rv.Field(i)
		if fv.Kind() == reflect.Struct {
			names = append(names, buildArgsIndex(flags, fv)...)
			continue
		}

		tag := ft.Tag.Get("cortana")
		parts := strings.Fields(tag)
		switch l := len(parts); {
		case l == 0:
			names = append(names, &fv)
		case l == 1:
			if parts[0] != "-" {
				if strings.HasPrefix(parts[0], "-") {
					flags[parts[0]] = &fv
				} else {
					names = append(names, &fv)
				}
			}
		case l >= 2:
			if parts[0] != "-" {
				flags[parts[0]] = &fv
			}
			if parts[1] != "-" {
				flags[parts[1]] = &fv
			}
		}
		// apply the default value
		if len(parts) >= 3 && parts[2] != "-" {
			applyValue(&fv, parts[2])
		}
	}
	return names
}
func applyValue(v *reflect.Value, s string) error {
	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Int, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		v.SetInt(i)
	case reflect.Uint, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return err
		}
		v.SetUint(u)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return err
		}
		v.SetFloat(f)
	}
	return nil
}

// UnmarshalArgs fills v with the parsed args
func UnmarshalArgs(args []string, v interface{}) {
	flags := make(map[string]*reflect.Value)
	names := buildArgsIndex(flags, reflect.ValueOf(v))

	var n int
	for i := 0; i < len(args); i++ {
		if !strings.HasPrefix(args[i], "-") {
			if n == len(names) {
				Fatal(errors.New("unknown argument " + args[i]))
			}
			if err := applyValue(names[n], args[i]); err != nil {
				Fatal(err)
			}
			n++
			continue
		}
		//TODO handle pattern: --key=value, --key
		flag, ok := flags[args[i]]
		if ok {
			if err := applyValue(flag, args[i+1]); err != nil {
				Fatal(err)
			}
			i++
		}
	}
}

var c *Cortana

func init() {
	c = New()
}

func Use(title, description string, v interface{}) {
	c.Use(title, description, v)
}

func Usage() {
	c.Usage()
}
func Args() []string {
	return c.Args()
}
func AddCommand(path string, cmd Command) {
	c.AddCommand(path, cmd)
}

func Launch() {
	c.Launch()
}
