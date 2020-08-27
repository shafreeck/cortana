# Cortana
An extremely easy to use command line parsing library 

## Codes
```go
func sayAnything() {
	title := "Say anything to anyone"
	description := `You can say anything you want to anyone, the person can be
selected by using the name, age or location. You can even combine these conditions
together to choose the person more effectively`
	greeting := struct { //format: --long -short defaultValue description
		Name     string `cortana:"--name -n cortana say something to cortana"`
		Age      int    `cortana:"--age - 18 say something to someone with certain age"`
		Location string `cortana:"--location -l beijing say something to someone lives in certain location"`
		Text     string `cortana:"text"`
	}{}
	cortana.Parse(&greeting,
		cortana.WithTitle(title),
		cortana.WithDescription(description))

	fmt.Printf("Say to %s who is %d year old and lives in %s now:\n",
		greeting.Name, greeting.Age, greeting.Location)
	fmt.Println(greeting.Text)

}
```

## Output

```
> ./example say -h
Say anything to anyone

You can say anything you want to anyone, the person can be
selected by using the name, age or location. You can even combine these conditions
together to choose the person more effectively

Available commands:

say hello                     say hello to anyone
say hello cortana             say hello to cortana

Usage: say [options] <text>

  -n, --name <name>              say something to cortana. (default=cortana)
      --age <age>                say something to someone with certain age. (default=18)
  -l, --location <location>      say something to someone lives in certain location. (default=beijing)

> ./example say --name alice "Hello world"
Say to alice who is 18 year old and lives in beijing now:
Hello world
```