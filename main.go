package main

import (
	"fmt"
	"github.com/glossd/yetis/client"
	"github.com/glossd/yetis/server"
	"os"
)

func main() {
	args := os.Args
	if len(args) < 2 {
		printHelp()
	}

	var needsServer = map[string]bool{
		"shutdown": true,
		"list":     true,
		"describe": true,
		"delete":   true,
		"apply":    true,
	}
	if needsServer[args[1]] {
		if !client.IsServerRunning() {
			fmt.Println("Yetis server isn't running")
			return
		}
	}

	switch args[1] {
	case "start":
		server.Start()
	case "shutdown":
		client.ShutdownServer()
	case "list":
		if len(args) == 2 {
			client.List()
			return
		}
		switch args[2] {
		case "-w":
			client.ListWatch()
		default:
			fmt.Print(`The flags for list command are:
	-w  watches for new updates
`)
		}
	case "describe":
		if len(os.Args) < 3 {
			fmt.Println("provide the name of the process")
			return
		}
		name := os.Args[2]
		client.Describe(name)
	case "delete":
		if len(os.Args) < 3 {
			fmt.Println("provide the name of the process")
			return
		}
		name := os.Args[2]
		client.Delete(name)
	case "apply":
		if len(os.Args) < 4 || os.Args[2] != "-f" {
			fmt.Println("expected command 'apply -f /path/to/config.yaml'")
			return
		}

		path := os.Args[3]
		client.Apply(path)
	case "help":
		printHelp()
	default:
		fmt.Printf("%s is not a valid command\n", args[1])
		printHelp()
	}
}

func printHelp() {
	fmt.Printf(`
The commands are:
	start            start Yetis server
	shutdown         terminate Yetis server
	apply -f [path]  create new deployments from the yaml configuration starting its processes.
	list             list the managed deployments
	describe [name]  get full info of the deployment 
	delete [name]    deletes the deployment terminating its process. 
	help             prints the list of the commands
`)
}
