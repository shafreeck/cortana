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
	flags := make(map[string]*flag)
	nonflags := buildArgsIndex(flags, reflect.ValueOf(v))

	w := bytes.NewBuffer(nil)
	w.WriteString(c.ctx.name)

	for _, nf := range nonflags {
		if nf.required {
			w.WriteString(" <" + nf.long + ">")
		} else {
			w.WriteString(" [" + nf.long + "]")
		}
	}
	w.WriteString("\n")

	for flag := range flags {
		w.WriteString(flag + "\n")
	}
	return w.String()
}

func buildArgsIndex(flags map[string]*flag, rv reflect.Value) []*nonflag {
	nonflags := make([]*nonflag, 0)
	for rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		ft := rt.Field(i)
		fv := rv.Field(i)
		if fv.Kind() == reflect.Struct {
			nonflags = append(nonflags, buildArgsIndex(flags, fv)...)
			continue
		}

		tag := ft.Tag.Get("cortana")
		f := parseFlag(tag, fv)
		if strings.HasPrefix(f.long, "-") {
			if f.long != "-" {
				flags[f.long] = f
			}
			if f.short != "-" {
				flags[f.short] = f
			}
		} else {
			nf := nonflag(*f)
			nonflags = append(nonflags, &nf)
		}
	}
	return nonflags
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
	flags := make(map[string]*flag)
	nonflags := buildArgsIndex(flags, reflect.ValueOf(v))

	var n int
	for i := 0; i < len(args); i++ {
		// handle nonflags
		if !strings.HasPrefix(args[i], "-") {
			if n == len(nonflags) {
				Fatal(errors.New("unknown argument " + args[i]))
			}
			if err := applyValue(&nonflags[n].rv, args[i]); err != nil {
				Fatal(err)
			}
			n++
			continue
		}
		//TODO handle flags pattern: --key value, --key=value, --key
		flag, ok := flags[args[i]]
		if ok {
			if err := applyValue(&flag.rv, args[i+1]); err != nil {
				Fatal(err)
			}
			i++
		} else {
			Fatal(errors.New("unknow argument " + args[i]))
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
