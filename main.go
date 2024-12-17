package main

import (
	"fmt"
	"github.com/glossd/yetis/client"
	"github.com/glossd/yetis/common"
	"github.com/glossd/yetis/server"
	"os"
	"time"
)

func main() {
	args := os.Args
	if len(args) < 2 {
		printHelp()
		return
	}

	var serverless = map[string]bool{
		"start": true,
		"run":   true,
		"help":  true,
	}
	if !serverless[args[1]] {
		if !client.IsServerRunning() {
			fmt.Println("Yetis server isn't running")
			return
		}
	}

	switch args[1] {
	case "info":
		client.Info()
		fmt.Printf("Client: version=%s\n", common.YetisVersion)
	case "run":
		// starts Yetis server in the foreground
		server.Run()
	case "start":
		logdir := "/tmp"
		if len(args) > 3 {
			if args[2] == "-d" {
				logdir = args[3]
			} else {
				printFlags("start", "-d  directory for the server log")
				return
			}
		}
		client.StartBackground(logdir)
	case "shutdown":
		client.ShutdownServer(5 * time.Minute)
	case "list":
		fallthrough
	case "get":
		if len(args) == 2 {
			client.GetDeployments()
			return
		}
		switch args[2] {
		case "-w":
			client.WatchGetDeployments()
		default:
			printFlags("list", "-w  watches for new updates")
			return
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
			client.Logs(os.Args[3], true)
		default:
			client.Logs(os.Args[2], false)
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

func printFlags(cmd string, flags ...string) {
	fmt.Printf("The flags for %s command are:\n", cmd)
	for _, flag := range flags {
		fmt.Println("	" + flag)
	}
}

func needName() {
	fmt.Println("provide the name of the deployment")
}

func printHelp() {
	fmt.Printf(`The commands are:
	start [-d]       start Yetis server
	shutdown         terminate Yetis server
	info             print server status
	apply -f {path}  create new deployments from yaml configuration starting its processes
	list [-w]        list the managed deployments
	logs [-f] {name} print the logs of a deployment
	describe {name}  get full info of the deployment
	delete {name}    delete the deployment terminating its process
	help             print the list of the commands
`)
}
