package cortana

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/google/btree"
)

// Cortana is the commander
type Cortana struct {
	ctx      context
	commands commands
	flags    struct { // the custom flags
		help longshort
		cfg  longshort
	}
	configs []*config
	envs    []EnvUnmarshaler
	seq     int
}

// fatal exit the process with an error
func fatal(err error) {
	fmt.Println(err)
	os.Exit(-1)
}

type Option func(ls *longshort)

func HelpFlag(long, short string) Option {
	return func(ls *longshort) {
		ls.long = long
		ls.short = short
		ls.desc = "help for the command"
	}
}
func DisableHelpFlag() Option {
	return HelpFlag("", "")
}

// New a Cortana commander
func New(opts ...Option) *Cortana {
	c := &Cortana{commands: commands{t: btree.New(8)}, ctx: context{args: os.Args[1:], name: os.Args[0]}}
	c.flags.help = longshort{
		long:  "--help",
		short: "-h",
		desc:  "help for the command",
	}
	for _, opt := range opts {
		opt(&c.flags.help)
	}
	return c
}

// AddCommand adds a command
func (c *Cortana) AddCommand(path string, cmd func(), brief string) {
	c.commands.t.ReplaceOrInsert(&command{Path: path, Proc: cmd, Brief: brief, order: c.seq})
	c.seq++
}

// AddRootCommand adds the command without sub path
func (c *Cortana) AddRootCommand(cmd func()) {
	c.AddCommand("", cmd, "")
}

type AddConfigOption Option

// ConfFlag parse the configration file path from flags
func ConfFlag(long, short string) AddConfigOption {
	return func(ls *longshort) {
		ls.long = long
		ls.short = short
		ls.desc = "path of the configuration file"
	}
}

// AddConfig adds a config file
func (c *Cortana) AddConfig(path string, unmarshaler Unmarshaler, opts ...AddConfigOption) {
	cfg := &config{path: path, unmarshaler: unmarshaler}
	for _, opt := range opts {
		opt(&c.flags.cfg)
	}

	c.configs = append(c.configs, cfg)
}

func (c *Cortana) AddEnvUnmarshaler(unmarshaler EnvUnmarshaler) {
	c.envs = append(c.envs, unmarshaler)
}

// Launch and run commands
func (c *Cortana) Launch() {
	cmd := c.searchCommand(os.Args[1:])
	if cmd == nil {
		c.Usage()
		return
	}
	cmd.Proc()
}

func (c *Cortana) searchCommand(args []string) *Command {
	var cmdArgs []string
	var maybeArgs []string
	var path string
	const (
		StateCommand = iota
		StateCommandPrefix
		StateOptionFlag
		StateOptionArg
		StateCommandArg
	)
	st := StateCommand
	cmd := c.commands.get(path)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch st {
		case StateCommand:
			if strings.HasPrefix(arg, "-") {
				st = StateOptionFlag
				cmdArgs = append(cmdArgs, arg)
				continue
			}
			p := strings.TrimSpace(path + " " + arg)
			commands := c.commands.scan(p)
			if len(commands) > 0 {
				path = p
				if commands[0].Path == path {
					maybeArgs = maybeArgs[:0]
					cmd = commands[0]
					st = StateCommand
					continue
				}
				maybeArgs = append(maybeArgs, arg)
				st = StateCommandPrefix
				continue
			}
			if cmd != nil {
				cmdArgs = append(cmdArgs, arg)
				st = StateCommandArg
				continue
			}
			fatal(errors.New("unknown command: " + p))

		case StateCommandPrefix:
			if strings.HasPrefix(arg, "-") {
				st = StateOptionFlag
				cmdArgs = append(cmdArgs, arg)
				continue
			}

			p := strings.TrimSpace(path + " " + arg)
			commands := c.commands.scan(p)
			if len(commands) > 0 {
				path = p
				if commands[0].Path == path {
					maybeArgs = maybeArgs[:0]
					cmd = commands[0]
					st = StateCommand
					continue
				}
				continue
			}

		case StateOptionFlag:
			if strings.HasPrefix(arg, "-") {
				cmdArgs = append(cmdArgs, arg)
				continue
			}

			p := strings.TrimSpace(path + " " + args[i])
			commands := c.commands.scan(p)
			if len(commands) > 0 {
				path = p
				if commands[0].Path == path {
					maybeArgs = maybeArgs[:0]
					cmd = commands[0]
					st = StateCommand
					continue
				}
				maybeArgs = append(maybeArgs, arg)
				st = StateCommandPrefix
				continue
			}
			cmdArgs = append(cmdArgs, arg)
			st = StateOptionArg

		case StateOptionArg:
			if strings.HasPrefix(arg, "-") {
				cmdArgs = append(cmdArgs, arg)
				st = StateOptionFlag
				continue
			}

			p := strings.TrimSpace(path + " " + args[i])
			commands := c.commands.scan(p)
			if len(commands) > 0 {
				path = p
				if commands[0].Path == path {
					maybeArgs = maybeArgs[:0]
					cmd = commands[0]
					st = StateCommand
					continue
				}
				maybeArgs = append(maybeArgs, arg)
				st = StateCommandPrefix
				continue
			}
			cmdArgs = append(cmdArgs, arg)
			st = StateCommandArg

		case StateCommandArg:
			if strings.HasPrefix(arg, "-") {
				cmdArgs = append(cmdArgs, arg)
				st = StateOptionFlag
				continue
			}
			cmdArgs = append(cmdArgs, arg)
		}
	}

	cmdArgs = append(cmdArgs, maybeArgs...)
	name := path
	if cmd != nil {
		name = cmd.Path
	}
	c.ctx = context{
		name:    name,
		args:    cmdArgs,
		longest: path,
	}
	return (*Command)(cmd)
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

// Parse the flags
func (c *Cortana) Parse(v interface{}) {
	if v == nil {
		return
	}
	for func() (restart bool) {
		defer func() {
			if v := recover(); v != nil {
				if s, ok := v.(string); ok && s == "restart" {
					restart = true
				} else {
					panic(v)
				}
			}
		}()
		c.collectFlags(v)
		c.applyDefaultValues(v)
		c.unmarshalConfigs(v)
		c.unmarshalEnvs(v)
		c.unmarshalArgs(v)
		c.checkRequires(v)
		return false
	}() {
	}
}

// Title set the title for the command
func (c *Cortana) Title(text string) {
	c.ctx.desc.title = text
}

// Description set the description for the command, it always be helpful
// to describe about the details of command
func (c *Cortana) Description(text string) {
	c.ctx.desc.description = text
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

	//  print the aliailable commands
	commands := c.commands.scan(c.ctx.longest)
	// ignore the command itself
	if len(commands) > 0 && commands[0].Path == c.ctx.name {
		commands = commands[1:]
	}
	if len(commands) > 0 {
		fmt.Println("Available commands:")
		fmt.Println()
		sort.Sort(orderedCommands(commands))

		cmds := bytes.NewBuffer(nil)
		alias := bytes.NewBuffer(nil)
		for _, cmd := range commands {
			writeString := cmds.WriteString
			if cmd.Alias {
				writeString = alias.WriteString
			}
			writeString(fmt.Sprintf("%-30s%s\n", cmd.Path, cmd.Brief))
		}
		fmt.Println(cmds.String())
		fmt.Println()
		if alias.Len() > 0 {
			fmt.Println("Alias commands:")
			fmt.Println()
			fmt.Println(alias.String())
		}
	}

	if c.ctx.desc.flags != "" {
		fmt.Println("Usage:", c.ctx.desc.flags)
	}
	os.Exit(0)
}

func (c *Cortana) Alias(name, definition string) {
	processAlias := func() {
		c.alias(definition)
	}
	alias := fmt.Sprintf("alias %-5s = %-20s", name, definition)
	c.commands.t.ReplaceOrInsert(&command{Path: name, Proc: processAlias, Brief: alias, order: c.seq, Alias: true})
	c.seq++
}
func (c *Cortana) alias(definition string) {
	args := strings.Fields(definition)
	cmd := c.searchCommand(append(args, c.ctx.args...))
	if cmd == nil {
		c.Usage()
		return
	}
	cmd.Proc()
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

	if c.flags.help.short != "" || c.flags.help.long != "" {
		flags = append(flags, &flag{
			long:        c.flags.help.long,
			short:       c.flags.help.short,
			description: c.flags.help.desc,
			required:    true,
			rv:          reflect.ValueOf(false),
		})
	}
	if c.flags.cfg.short != "" || c.flags.cfg.long != "" {
		conf := c.configs[len(c.configs)-1]
		required := false
		if conf.path == "" {
			required = true
		}
		flags = append(flags, &flag{
			long:         c.flags.cfg.long,
			short:        c.flags.cfg.short,
			description:  c.flags.cfg.desc,
			required:     required,
			defaultValue: conf.path,
		})
	}
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
		if tag == "" {
			tag = ft.Tag.Get("lsdd") // lsdd is short for (long short default description)
		}
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
		if args[i] == c.flags.help.long || args[i] == c.flags.help.short {
			c.Usage()
			return
		}
		// handle nonflags
		if !strings.HasPrefix(args[i], "-") && len(nonflags) > 0 {
			if err := applyValue(nonflags[0].rv, args[i]); err != nil {
				fatal(err)
			}
			nonflags = nonflags[1:]
			continue
		}

		var key, value string
		if strings.Index(args[i], "=") > 0 {
			kvs := strings.SplitN(args[i], "=", 2)
			key, value = kvs[0], kvs[1]
		} else {
			key = args[i]
		}

		// handle the config flags
		if len(c.configs) > 0 &&
			(key == c.flags.cfg.long || key == c.flags.cfg.short) {
			cfg := c.configs[len(c.configs)-1] // overwrite the last one
			if value != "" {
				cfg.path = value
				c.ctx.args = append(args[0:i], args[i+1:]...)
				panic("restart")
			} else if i+1 < len(args) {
				cfg.path = args[i+1]
				c.ctx.args = append(args[0:i], args[i+2:]...)
				panic("restart")
			}
			fatal(errors.New(key + " requires an argument"))
		}

		flag, ok := flags[key]
		if ok {
			if value != "" {
				if err := applyValue(flag.rv, value); err != nil {
					fatal(err)
				}
				continue
			}
			if flag.rv.Kind() == reflect.Bool {
				if err := applyValue(flag.rv, "true"); err != nil {
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
				}
				continue
			}
			fatal(errors.New(key + " requires an argument"))
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

func (c *Cortana) unmarshalEnvs(v interface{}) {
	for _, u := range c.envs {
		if err := u.Unmarshal(v); err != nil {
			fatal(err)
		}
	}
}

var c *Cortana

func init() {
	c = New()
}

// Parse the arguemnts into a struct
func Parse(v interface{}) {
	c.Parse(v)
}

// Title set the title for the command
func Title(text string) {
	c.Title(text)
}

// Description set the description for the command, it always be helpful
// to describe about the details of command
func Description(text string) {
	c.Description(text)
}

// Usage prints the usage and exits
func Usage() {
	c.Usage()
}

// Alias gives another name for command. Ex. cortana.Alias("rmi", "rm -i")
func Alias(name, definition string) {
	c.Alias(name, definition)
}

// Args returns the arguments for current command
func Args() []string {
	return c.Args()
}

// AddCommand adds a command
func AddCommand(path string, cmd func(), brief string) {
	c.AddCommand(path, cmd, brief)
}

// AddRootCommand adds the command without sub path
func AddRootCommand(cmd func()) {
	c.AddRootCommand(cmd)
}

// AddConfig adds a configuration file
func AddConfig(path string, unmarshaler Unmarshaler, opts ...AddConfigOption) {
	c.AddConfig(path, unmarshaler, opts...)
}

// Commands returns the list of the added commands
func Commands() []*Command {
	return c.Commands()
}

// Launch finds and executes the command
func Launch() {
	c.Launch()
}
