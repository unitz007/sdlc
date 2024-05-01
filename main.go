package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"sdlc/models"
	"strings"

	"github.com/spf13/cobra"
)

func main() {

	argCommand := ""

	rootCmd := models.NewCommand("sdlc", "", func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			_ = cmd.Help()
			return
		}
	}).Cmd

	subCommands := []struct {
		command     string
		description string
		action      func(cmd *cobra.Command, args []string)
	}{
		{
			"run",
			"Runs your code",
			func(cmd *cobra.Command, args []string) {
				argCommand = "run"
			},
		}, {
			"test",
			"Tests your code",
			func(cmd *cobra.Command, args []string) {
				argCommand = "test"
			},
		}, {
			"build",
			"Builds your project",
			func(cmd *cobra.Command, args []string) {
				argCommand = "build"
			},
		},
	}

	for _, subCommand := range subCommands {
		s := models.NewCommand(subCommand.command, subCommand.description, subCommand.action)
		rootCmd.AddCommand(s.Cmd)
	}

	err := rootCmd.Execute()

	if err != nil || argCommand == "" {
		os.Exit(1)
	}

	configFile, err := decodeConfig()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	buildData := &configFile.Builds
	commands := *buildData
	var buildFile string
	var task *models.Task
	var command string
	var program string
	workingDirectory, _ := os.Getwd()

	for _, c := range commands {
		buildFile = c.BuildFile()

		filesInWorkingDirectory, _ := ioutil.ReadDir(workingDirectory)

		for _, file := range filesInWorkingDirectory {
			if file.Name() == buildFile {
				task = c.Task()
			}
		}
	}

	//fmt.Println(buildFile)

	if task != nil {
		command = task.Command(argCommand)
		program = task.Program
	} else {
		fmt.Println("project not configured on sdlc-config file")
		return
	}

	if command == "" {
		panic("invalid command")
	}

	fmt.Printf("Executing command: %s %s\n", program, command)

	com := exec.Command(program, strings.Split(command, " ")...)

	stdOut, err := com.StdoutPipe()

	if err := com.Start(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	buf := bufio.NewReader(stdOut)
	line, err := buf.ReadString('\n')
	for err == nil {
		fmt.Print(line)
		line, err = buf.ReadString('\n')
	}
	//num := 0
	//
	//for {
	//	line, _, _ := buf.ReadLine()
	//	//if num > 100 {
	//	//	os.Exit(1)
	//	//}
	//	//num += 1
	//	fmt.Println(string(line))
	//}

}

type ConfigFile struct {
	data string
}

func loadConfig(data string) (*models.Build, error) {

	var build models.Build

	err := json.Unmarshal([]byte(data), &build)

	if err != nil {
		return &models.Build{}, fmt.Errorf("error: config content should be in json format")
	}

	return &build, nil
}

func decodeConfig() (*models.Build, error) {
	homePath, err := os.UserHomeDir()
	configFile := path.Join(homePath, ".sdlc-config.json")

	//currentPathNew := strings.ReplaceAll(homePath, ".sdlc", "")

	fileContent, err := os.ReadFile(configFile)
	if err != nil {
		////fmt.Println("config file not found:", currentPathNew)
		//currentPathNew = os.Getenv("SDLC_CONFIG_FILE")
		//fileContent, err = os.ReadFile(currentPathNew + "/sdlc-config.json")
		if err != nil {
			_, err := os.Create(configFile)
			if err != nil {
				fmt.Println("could not create config file:", err.Error())
				os.Exit(1)
			}
		}
	}

	jsonFile := string(fileContent)

	build, err := loadConfig(jsonFile)

	if err != nil {
		return &models.Build{}, fmt.Errorf("error:  invalid config structure")
	}

	return build, nil
}
