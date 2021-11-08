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
		Location string `cortana:"--location, -l, beijing, say something to someone lives in certain location"`
		Text     string `cortana:"text, -, -"`
	}{}

	var age int
	cortana.Flag(&age, "--age", "-", "18", "say something to someone with certain age")
	cortana.Parse(&greeting)

	fmt.Printf("Say to %s who is %d year old and lives in %s now:\n",
		greeting.Name, age, greeting.Location)
	fmt.Println(greeting.Text)

}

func main() {
	cortana.Command("say hello cortana", "say hello to cortana", sayHelloCortana)
	cortana.Command("say hello", "say hello to anyone", sayHelloAnyone)
	cortana.Command("say", "say anything to anyone", sayAnything)

	cortana.Alias("cortana", "say hello cortana")

	cortana.Config("greeting.json", cortana.UnmarshalFunc(json.Unmarshal))
	cortana.Use(cortana.ConfFlag("--config", "-c", cortana.UnmarshalFunc(json.Unmarshal)))

	cortana.Launch()
}
