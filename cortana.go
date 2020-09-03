package cortana

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
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
	configs  []*config
}

// fatal exit the process with an error
func fatal(err error) {
	fmt.Println(err)
	os.Exit(-1)
}

// New a Cortana commander
func New() *Cortana {
	return &Cortana{commands: commands{t: btree.New(8)}}
}

// AddCommand adds a command
func (c *Cortana) AddCommand(path string, cmd func(), brief string) {
	c.commands.t.ReplaceOrInsert(&command{Path: path, Proc: cmd, Brief: brief})
}

// AddConfig adds a config file
func (c *Cortana) AddConfig(path string, unmarshaler Unmarshaler) {
	c.configs = append(c.configs, &config{path: path, unmarshaler: unmarshaler})
}

// Launch and run commands
func (c *Cortana) Launch() {
	args := os.Args[1:]

	// the arguments with '-' prefix are flags, others are names
	var names []string
	var flags []string
	for i := 0; i < len(args); i++ {
		if args[i][0] == '-' {
			flags = append(flags, args[i])
			if i+1 < len(args) {
				if args[i+1][0] == '-' {
					continue
				}
				flags = append(flags, args[i+1])
				i++
			}
		} else {
			names = append(names, args[i])
		}
	}

	// no sub commands
	l := len(names)
	if l == 0 {
		cmd := c.commands.get("")
		if cmd != nil {
			cmd.Proc()
		} else {
			c.Usage()
		}
	}

	// search for the command
	for i := range names {
		path := strings.Join(names[0:l-i], " ")
		commands := c.commands.scan(path)
		if len(commands) == 0 {
			// no more commands in path
			if i+1 == l {
				fatal(errors.New("unknown command pattern: " + strings.Join(names, " ")))
			}
		} else {
			cmd := commands[0]
			if cmd.Path == path {
				args := append(names[l-i:], flags...)
				c.ctx = context{
					name: path,
					args: args,
				}
				commands[0].Proc()
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

// Commands returns all the available commands
func (c *Cortana) Commands() []*Command {
	var commands []*Command

	// scan all the commands
	cmds := c.commands.scan("")
	for _, c := range cmds {
		commands = append(commands, (*Command)(c))
	}
	return commands
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
	c.collectFlags(v)
	c.applyDefaultValues(v)
	c.unmarshalConfigs(v)
	c.unmarshalArgs(v)
	c.checkRequires(v)
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
	// ignore the command itself
	if len(commands) > 0 && commands[0].Path == c.ctx.name {
		commands = commands[1:]
	}
	if len(commands) > 0 {
		fmt.Println("Available commands:")
		fmt.Println()
		for _, cmd := range commands {
			fmt.Printf("%-30s%s\n", cmd.Path, cmd.Brief)
		}
		fmt.Println()
	}

	if c.ctx.desc.flags != "" {
		fmt.Println("Usage:", c.ctx.desc.flags)
	}
	os.Exit(0)
}

func (c *Cortana) collectFlags(v interface{}) {
	flags, nonflags := parseCortanaTags(reflect.ValueOf(v))

	w := bytes.NewBuffer(nil)
	w.WriteString(c.ctx.name)
	if len(flags) > 0 {
		w.WriteString(" [options]")
	}
	for _, nf := range nonflags {
		if nf.required {
			w.WriteString(" <" + nf.long + ">")
		} else {
			w.WriteString(" [" + nf.long + "]")
		}
	}
	w.WriteString("\n\n")

	for _, f := range flags {
		var flag string
		if f.short != "-" {
			flag += f.short
		}
		if f.long != "-" {
			if f.short != "-" {
				flag += ", " + f.long
			} else {
				flag += "    " + f.long
			}
		}
		if f.rv.Kind() != reflect.Bool {
			if f.long != "-" {
				flag += " <" + strings.TrimLeft(f.long, "-") + ">"
			} else {
				flag += " <" + strings.ToLower(f.name) + ">"
			}
		}
		if len(flag) > 30 {
			// align with 32 spaces
			flag += "\n                                "
		}
		if !f.required {
			s := fmt.Sprintf("  %-30s %s. (default=%s)\n", flag, f.description, f.defaultValue)
			w.WriteString(s)
		} else {
			s := fmt.Sprintf("  %-30s %s\n", flag, f.description)
			w.WriteString(s)
		}
	}

	c.ctx.desc.flags = w.String()
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
		f := parseFlag(tag, ft.Name, fv)
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
func (c *Cortana) applyDefaultValues(v interface{}) {
	flags, nonflags := parseCortanaTags(reflect.ValueOf(v))
	for _, nf := range nonflags {
		if nf.required {
			continue
		}
		if err := applyValue(nf.rv, nf.defaultValue); err != nil {
			fatal(err)
		}
	}
	for _, f := range flags {
		if f.required {
			continue
		}
		if f.rv.Kind() == reflect.Slice && f.defaultValue == "nil" {
			continue
		}
		if err := applyValue(f.rv, f.defaultValue); err != nil {
			fatal(err)
		}
	}
}
func applyValue(v reflect.Value, s string) error {
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
	case reflect.Slice:
		e := reflect.New(v.Type().Elem()).Elem()
		if err := applyValue(e, s); err != nil {
			return err
		}
		v.Set(reflect.Append(v, e))
	}
	return nil
}
func (c *Cortana) checkRequires(v interface{}) {
	flags, nonflags := parseCortanaTags(reflect.ValueOf(v))

	args := c.ctx.args
	// check the nonflags
	i := 0
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			break
		}
		i++
	}
	if i < len(nonflags) {
		for _, nf := range nonflags[i:] {
			if nf.required && nf.rv.IsZero() {
				fatal(errors.New("<" + nf.long + "> is required"))
			}
		}

	}

	// check the flags
	argsIdx := make(map[string]struct{})
	for _, arg := range args {
		argsIdx[arg] = struct{}{}
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
		if !f.rv.IsZero() {
			continue
		}

		if f.long != "-" {
			fatal(errors.New(f.long + " is required"))
		}
		if f.short != "-" {
			fatal(errors.New(f.short + " is required"))
		}
	}
}

// unmarshalArgs fills v with the parsed args
func (c *Cortana) unmarshalArgs(v interface{}) {
	flags := make(map[string]*flag)
	nonflags := buildArgsIndex(flags, reflect.ValueOf(v))

	args := c.ctx.args
	for i := 0; i < len(args); i++ {
		// print the usage and exit
		if args[i] == "-h" || args[i] == "--help" {
			c.Usage()
		}
		// handle nonflags
		if !strings.HasPrefix(args[i], "-") {
			if len(nonflags) == 0 {
				fatal(errors.New("unknown argument: " + args[i]))
			}
			if err := applyValue(nonflags[0].rv, args[i]); err != nil {
				fatal(err)
			}
			nonflags = nonflags[1:]
			continue
		}

		var key, value string
		if strings.Index(args[i], "=") > 0 {
			kvs := strings.SplitN(args[i], "=", 1)
			key, value = kvs[0], kvs[1]
		} else {
			key = args[i]
		}
		flag, ok := flags[key]
		if ok {
			if value != "" {
				if err := applyValue(flag.rv, value); err != nil {
					fatal(err)
				}
				continue
			}
			if i+1 < len(args) {
				next := args[i+1]
				if next[0] != '-' {
					if err := applyValue(flag.rv, next); err != nil {
						fatal(err)
					}
					i++
					continue
				}
			}
			if flag.rv.Kind() == reflect.Bool {
				if err := applyValue(flag.rv, "true"); err != nil {
					fatal(err)
				}
			} else {
				fatal(errors.New(key + " requires an argument"))
			}
		} else {
			fatal(errors.New("unknown argument: " + args[i]))
		}
	}
}

func (c *Cortana) unmarshalConfigs(v interface{}) {
	for _, cfg := range c.configs {
		file, err := os.Open(cfg.path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			fatal(err)
		}
		data, err := ioutil.ReadAll(file)
		if err != nil {
			fatal(err)
		}

		if err := cfg.unmarshaler.Unmarshal(data, v); err != nil {
			fatal(err)
		}
		file.Close()
	}
}

var c *Cortana

func init() {
	c = New()
}

// Parse the arguemnts into a struct
func Parse(v interface{}, opts ...ParseOption) {
	c.Parse(v, opts...)
}

// Usage prints the usage and exits
func Usage() {
	c.Usage()
}

// Args returns the arguments for current command
func Args() []string {
	return c.Args()
}

// AddCommand adds a command
func AddCommand(path string, cmd func(), brief string) {
	c.AddCommand(path, cmd, brief)
}

// AddConfig adds a configuration file
func AddConfig(path string, unmarshaler Unmarshaler) {
	c.AddConfig(path, unmarshaler)
}

// Commands returns the list of the added commands
func Commands() []*Command {
	return c.Commands()
}

// Launch finds and executes the command
func Launch() {
	c.Launch()
}
