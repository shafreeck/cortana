# Cortana
An extremely easy to use command line parsing library 

## How to use

### Work in the main function

```go
// say.go
func main() {
	args := struct {
		Name string `cortana:"--name, -n, tom, who do you want say to"`
		Text string `cortana:"text"`
	}{}
	// Parse the args
	cortana.Parse(&args)
	fmt.Printf("Say to %s: %s\n", args.Name, args.Text)
}

$ go run say.go -n alice hello
Say to alice: hello
```

### Use as the sub-command

```go
// pepole.go
func say() {
	args := struct {
		Name string `cortana:"--name, -n, tom, who do you want say to"`
		Text string `cortana:"text"`
	}{}
	cortana.Parse(&args)
	fmt.Printf("Say to %s: %s\n", args.Name, args.Text)
}

func main() {
	// Add "say" as a sub-command
	cortana.AddCommand("say", say, "say something")
	cortana.Launch()
}

$ go run pepole.go say -n alice hello
Say to alice: hello
```

### You defines how the sub-commond looks like without affecting the original implementation

```go
// people.go
func main() {
	// say greeting works by calling the say function
	cortana.AddCommand("say greeting", say, "say something")
	// greeting say also works by calling the say function
	cortana.AddCommand("greeting say", say, "say something")
	cortana.Launch()
}

$ go run pepole.go say greeting -n alice hello
Say to alice: hello

$ go run pepole.go greeting say -n alice hello
Say to alice: hello
```