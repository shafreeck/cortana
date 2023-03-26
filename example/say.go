package main

import (
	"encoding/json"
	"fmt"

	"github.com/shafreeck/cortana"
)

func sayHelloCortana() {
	fmt.Println("hello cortana")
}
func sayHelloAnyone() {
	person := struct {
		Name string `cortana:"name"`
	}{}
	cortana.Parse(&person)

	fmt.Println("hello", person.Name)
}
func sayAnything() {
	cortana.Title("Say anything to anyone")
	cortana.Description(`You can say anything you want to anyone, the person can be
selected by using the name, age or location. You can even combine these conditions
together to choose the person more effectively`)
	greeting := struct { //format: --long -short defaultValue description
		Name     string `lsdd:"--name, -n, cortana, say something to cortana"`
		Age      int    `cortana:"--age, -, 18, say something to someone with certain age"`
		Location string `cortana:"--location, -l, beijing, say something to someone lives in certain location"`
		Text     string `cortana:"text, -, -"`
	}{}
	cortana.Parse(&greeting)

	fmt.Printf("Say to %s who is %d year old and lives in %s now:\n",
		greeting.Name, greeting.Age, greeting.Location)
	fmt.Println(greeting.Text)

}

func complete() {
	cortana.Title("Complete a command")
	cortana.Description("return all the commands that has prefix")
	opts := struct {
		Prefix string `cortana:"prefix"`
	}{}
	cortana.Parse(&opts)

	cmds := cortana.Complete(opts.Prefix)
	for _, cmd := range cmds {
		fmt.Println(cmd.Path+":", cmd.Brief)
	}
}

func main() {
	cortana.AddCommand("say hello cortana", sayHelloCortana, "say hello to cortana")
	cortana.AddCommand("say hello", sayHelloAnyone, "say hello to anyone")
	cortana.AddCommand("say", sayAnything, "say anything to anyone")
	cortana.AddCommand("complete", complete, "complete a command prefix")

	cortana.Alias("cortana", "say hello cortana")

	cortana.AddConfig("greeting.json", cortana.UnmarshalFunc(json.Unmarshal))
	cortana.Use(cortana.ConfFlag("--config", "-c", cortana.UnmarshalFunc(json.Unmarshal)))

	cortana.Launch()
}
