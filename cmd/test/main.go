package main

import (
	survey "github.com/AlecAivazis/survey/v2"
)

type Answers struct {
	Name     string
	Favorite string
}

func main() {
	var answers Answers

	questions := []*survey.Question{
		{
			Name:      "name",
			Prompt:    &survey.Input{Message: "What is your name?"},
			Validate:  survey.Required,
			Transform: survey.Title,
		},
		{
			Name: "favorite",
			Prompt: &survey.Select{
				Message: "Choose a color:",
				Options: []string{"red", "blue", "green"},
				Default: "red",
			},
		},
	}

	err := survey.Ask(questions, &answers)
	if err != nil {
		// handle error
		panic(err)
	}

	println("Name : ", answers.Name)
	println("Favorite Color : ", answers.Favorite)
}
