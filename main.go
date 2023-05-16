package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sdlc/models"
	"strings"
)

func main() {

	args := os.Args[1:]
	if len(args) < 1 {
		panic("At least one command must be specified")
	}

	argCommand := args[0]

	configFile, err := decodeConfig()
	if err != nil {
		panic(err)
	}

	buildData := &configFile.Builds
	commands := *buildData
	var buildFile string
	var task *models.Task
	var command string
	var program string
	workingDirectory, _ := os.Getwd()

	for _, c := range commands {
		buildFile = c.BuildFile

		filesInWorkingDirectory, _ := ioutil.ReadDir(workingDirectory)

		for _, file := range filesInWorkingDirectory {
			if file.Name() == buildFile {
				task = &c.Task
			}
		}
	}

	if task != nil {
		command = task.Command(argCommand)
		program = task.Program
	} else {
		fmt.Println("project not configured on sdlc-config.json")
		return
	}

	if command == "" {
		panic("invalid command")
	}

	fmt.Printf("Executing command: %s %s\n", program, command)

	out, err := exec.Command(program, command).Output()
	if err != nil {
		fmt.Print(err)
	}

	output := string(out[:])

	fmt.Println(output)

}

type ConfigFile struct {
	data string
}

func loadConfig(data string) (*models.Build, error) {

	var build models.Build

	err := json.Unmarshal([]byte(data), &build)

	if err != nil {
		return nil, fmt.Errorf("error: config content should be in json format")
	}

	return &build, nil
}

func decodeConfig() (*models.Build, error) {
	currentPath, err := os.Executable()

	currentPathNew := strings.ReplaceAll(currentPath, "sdlc", "")

	fileContent, err := os.ReadFile(currentPathNew + "sdlc-config.json")
	if err != nil {
		panic(err)
	}

	jsonFile := string(fileContent)

	build, err := loadConfig(jsonFile)

	if err != nil {
		return nil, fmt.Errorf("error:  invalid config structure")
	}

	return build, nil
}
