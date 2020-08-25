package cortana

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/google/btree"
)

// Cortana is the commander
type Cortana struct {
	ctx      context
	commands commands
}

// Fatal exit the process with an error
func Fatal(err error) {
	fmt.Println(err)
	os.Exit(-1)
}

// New a Cortana commander
func New() *Cortana {
	return &Cortana{commands: commands{t: btree.New(8)}}
}

// AddCommand adds a command
func (c *Cortana) AddCommand(path string, cmd func(), brief string) {
	c.commands.t.ReplaceOrInsert(&command{path: path, brief: brief, proc: cmd})
}

// Launch and run commands
func (c *Cortana) Launch() {
	args := os.Args[1:]
	if len(args) == 0 {
		cmd := c.commands.get("")
		if cmd != nil {
			cmd.proc()
		} else {
			c.Usage()
		}
		return
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
		commands := c.commands.scan(path)
		if len(commands) == 0 {
			// no more commands in path
			if i+1 == l {
				Fatal(errors.New("unknown command pattern: " + strings.Join(names, " ")))
			}
		} else {
			cmd := commands[0]
			if cmd.path == path {
				args := append(names[l-i:], flags...)
				c.ctx = context{
					name: path,
					args: args,
				}
				commands[0].proc()
			} else {
				c.ctx = context{
					name: path,
				}
				c.Usage()
			}
			return
		}
	}
}

// Args returns the args in current context
func (c *Cortana) Args() []string {
	return c.ctx.args
}

// ParseOption is the option for Parse
type ParseOption func(d *desc)

// WithTitle parses arguments with title
func WithTitle(s string) ParseOption {
	return func(d *desc) {
		d.title = s
	}
}

// WithDescription pares arguments with description
func WithDescription(s string) ParseOption {
	return func(d *desc) {
		d.description = s
	}
}

// Parse the flags
func (c *Cortana) Parse(v interface{}, opts ...ParseOption) {
	for _, opt := range opts {
		opt(&c.ctx.desc)
	}
	if v == nil {
		return
	}
	c.ctx.desc.flags = c.collectFlags(v)
	c.unmarshalArgs(c.ctx.args, v)
	c.checkRequires(c.ctx.args, v)
}

// Usage prints the usage
func (c *Cortana) Usage() {
	if c.ctx.desc.title != "" {
		fmt.Println(c.ctx.desc.title)
		fmt.Println()
	}
	if c.ctx.desc.description != "" {
		fmt.Println(c.ctx.desc.description)
		fmt.Println()
	}
	commands := c.commands.scan(c.ctx.name)
	if len(commands) > 0 {
		fmt.Println("Available commands:")
		fmt.Println()
		for _, cmd := range commands {
			fmt.Printf("%-30s%s\n", cmd.path, cmd.brief)
		}
		fmt.Println()
	}

	if c.ctx.desc.flags != "" {
		fmt.Println("Usage:", c.ctx.desc.flags)
	}
	os.Exit(0)
}

func (c *Cortana) collectFlags(v interface{}) string {
	flags, nonflags := parseCortanaTags(reflect.ValueOf(v))

	w := bytes.NewBuffer(nil)
	w.WriteString(c.ctx.name)
	for _, nf := range nonflags {
		if nf.required {
			w.WriteString(" <" + nf.long + ">")
		} else {
			w.WriteString(" [" + nf.long + "]")
		}
	}
	if len(flags) > 0 {
		w.WriteString(" [options]\n\n")
	}

	for _, f := range flags {
		long := ""
		short := ""
		if f.long != "-" {
			long = f.long
		}
		if f.short != "-" {
			short = f.short
		}
		if !f.required {
			s := fmt.Sprintf("  %-2s %-20s %s. (default=%s)\n", short, long, f.description, f.defaultValue)
			w.WriteString(s)
		} else {
			s := fmt.Sprintf("  %-2s %-20s %s\n", short, long, f.description)
			w.WriteString(s)
		}
	}

	return w.String()
}

func parseCortanaTags(rv reflect.Value) ([]*flag, []*nonflag) {
	flags := make([]*flag, 0)
	nonflags := make([]*nonflag, 0)
	for rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		ft := rt.Field(i)
		fv := rv.Field(i)
		if fv.Kind() == reflect.Struct {
			f, nf := parseCortanaTags(fv)
			flags = append(flags, f...)
			nonflags = append(nonflags, nf...)
			continue
		}

		tag := ft.Tag.Get("cortana")
		f := parseFlag(tag, fv)
		if strings.HasPrefix(f.long, "-") {
			if f.long != "-" || f.short != "-" {
				flags = append(flags, f)
			}
		} else {
			nf := nonflag(*f)
			nonflags = append(nonflags, &nf)
		}
	}
	return flags, nonflags
}
func buildArgsIndex(flagsIdx map[string]*flag, rv reflect.Value) []*nonflag {
	flags, nonflags := parseCortanaTags(rv)
	if err := applyDefaultValues(flags, nonflags); err != nil {
		Fatal(err)
	}
	for _, f := range flags {
		if f.long != "" {
			flagsIdx[f.long] = f
		}
		if f.short != "" {
			flagsIdx[f.short] = f
		}
	}
	return nonflags
}
func applyDefaultValues(flags []*flag, nonflags []*nonflag) error {
	for _, nf := range nonflags {
		if nf.required {
			continue
		}
		if err := applyValue(&nf.rv, nf.defaultValue); err != nil {
			return err
		}
	}
	for _, f := range flags {
		if f.required {
			continue
		}
		if err := applyValue(&f.rv, f.defaultValue); err != nil {
			return err
		}
	}
	return nil
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
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return err
		}
		v.SetBool(b)
	}
	return nil
}
func (c *Cortana) checkRequires(args []string, v interface{}) {
	flags, nonflags := parseCortanaTags(reflect.ValueOf(v))

	argsIdx := make(map[string]struct{})
	for _, arg := range args {
		argsIdx[arg] = struct{}{}
	}
	for _, nf := range nonflags {
		if !nf.required {
			continue
		}
		if _, ok := argsIdx[nf.long]; !ok {
			Fatal(errors.New(nf.long + " is required"))
		}
	}
	for _, f := range flags {
		if !f.required {
			continue
		}
		if _, ok := argsIdx[f.long]; ok {
			continue
		}
		if _, ok := argsIdx[f.short]; ok {
			continue
		}

		if f.long != "-" {
			Fatal(errors.New(f.long + " is required"))
		}
		if f.short != "-" {
			Fatal(errors.New(f.short + " is required"))
		}
	}
}

// unmarshalArgs fills v with the parsed args
func (c *Cortana) unmarshalArgs(args []string, v interface{}) {
	flags := make(map[string]*flag)
	nonflags := buildArgsIndex(flags, reflect.ValueOf(v))

	for i := 0; i < len(args); i++ {
		// print the usage and exit
		if args[i] == "-h" || args[i] == "--help" {
			c.Usage()
		}
		// handle nonflags
		if !strings.HasPrefix(args[i], "-") {
			if len(nonflags) == 0 {
				Fatal(errors.New("unknown argument: " + args[i]))
			}
			if err := applyValue(&nonflags[0].rv, args[i]); err != nil {
				Fatal(err)
			}
			nonflags = nonflags[1:]
			continue
		}

		var key, value string
		if strings.Index(args[i], "=") > 0 {
			kvs := strings.Split(args[i], "=")
			key, value = kvs[0], kvs[1]
		} else {
			key = args[i]
		}
		flag, ok := flags[key]
		if ok {
			if value != "" {
				if err := applyValue(&flag.rv, value); err != nil {
					Fatal(err)
				}
				continue
			}
			if i+1 < len(args) {
				next := args[i+1]
				if next[0] != '-' {
					if err := applyValue(&flag.rv, next); err != nil {
						Fatal(err)
					}
					i++
					continue
				}
			}
			if flag.rv.Kind() == reflect.Bool {
				if err := applyValue(&flag.rv, "true"); err != nil {
					Fatal(err)
				}
			} else {
				Fatal(errors.New(key + " requires an argument"))
			}
		} else {
			Fatal(errors.New("unknown argument: " + args[i]))
		}
	}
}

var c *Cortana

func init() {
	c = New()
}

func Parse(v interface{}, opts ...ParseOption) {
	c.Parse(v, opts...)
}

func Usage() {
	c.Usage()
}
func Args() []string {
	return c.Args()
}
func AddCommand(path string, cmd func(), brief string) {
	c.AddCommand(path, cmd, brief)
}

func Launch() {
	c.Launch()
}
