package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sdlc/models"
)

func main() {

	// read json file
	file, err := os.ReadFile("./sdlc-config.json")
	if err != nil {
		panic(err)
	}

	jsonFile := string(file)

	task, err := loadConfig(jsonFile)

	if err != nil {
		panic(err)
	}

	fmt.Println(task.Run)

}

type ConfigFile struct {
	data string
}

func loadConfig(data string) (*models.Task, error) {

	var task models.Task

	err := json.Unmarshal([]byte(data), &task)

	if err != nil {
		return nil, fmt.Errorf("error: config content should be in json format")
	}

	return &task, nil
}
