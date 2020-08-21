package main

import (
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

func main() {
	cortana.AddCommand("say hello cortana", sayHelloCortana)
	cortana.AddCommand("say hello", sayHelloAnyone)
	cortana.AddCommand("say", sayAnything)
	cortana.Launch()
}
