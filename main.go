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

	var serverless = map[string]bool{
		"start": true,
		"help":  true,
	}
	if !serverless[args[1]] {
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
	case "logs":
		if len(os.Args) < 3 {
			needName()
			return
		}

		switch os.Args[2] {
		case "-f":
			if len(os.Args) < 3 {
				needName()
				return
			}
			client.LogsStream(os.Args[3])
		default:
			client.Logs(os.Args[2])
		}
	case "describe":
		if len(os.Args) < 3 {
			needName()
			return
		}
		name := os.Args[2]
		client.Describe(name)
	case "delete":
		if len(os.Args) < 3 {
			needName()
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

func needName() {
	fmt.Println("provide the name of the deployment")
}

func printHelp() {
	fmt.Printf(`
The commands are:
	start            start Yetis server
	shutdown         terminate Yetis server
	apply -f {path}  create new deployments from yaml configuration starting its processes
	list [-w]        list the managed deployments
	logs [-f] {name} print the logs of a deployment
	describe {name}  get full info of the deployment
	delete {name}    delete the deployment terminating its process
	help             print the list of the commands
`)
}
