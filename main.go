package main

import (
	"fmt"
	"github.com/glossd/yetis/client"
	"github.com/glossd/yetis/common"
	"github.com/glossd/yetis/server"
	"os"
	"strconv"
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
		if len(args) == 2 {
			client.ShutdownServer(5 * time.Minute)
			return
		}
		secondsStr := args[2]
		seconds, err := strconv.Atoi(secondsStr)
		if err != nil {
			fmt.Println("second argument should be the timeout in seconds")
		}
		client.ShutdownServer(time.Duration(seconds) * time.Second)
	case "list": // deprecated.
		fallthrough
	case "get":
		if len(args) == 2 {
			client.GetDeployments()
			return
		}
		switch args[2] {
		case "-w":
			if len(args) == 3 {
				client.WatchGetDeployments()
				return
			}
			switch args[3] {
			case "deployment", "d":
				client.WatchGetDeployments()
			case "service", "s":
				client.WatchGetServices()
			default:
				availableKinds()
			}
		case "deployment", "d":
			client.GetDeployments()
		case "service", "s":
			client.GetServices()
		default:
			printFlags("get [-flags] [kind]", "-w  watches for new updates")
			return
		}
	case "logs":
		if len(os.Args) < 3 {
			needName()
			return
		}

		switch os.Args[2] {
		case "-f":
			if len(os.Args) < 4 {
				needName()
				return
			}
			client.Logs(os.Args[3], true)
		default:
			client.Logs(os.Args[2], false)
		}
	case "describe":
		if len(os.Args) < 4 {
			fmt.Println("Invalid command, expected: describe [kind] [name]")
			return
		}
		switch os.Args[2] {
		case "service", "s":
			client.DescribeService(os.Args[3])
		case "deployment", "d":
			client.DescribeDeployment(os.Args[3])
		default:
			availableKinds()
		}
	case "delete":
		if len(os.Args) < 4 {
			fmt.Println("Invalid command, expected: delete [kind] [name]")
			return
		}
		switch os.Args[2] {
		case "service", "s":
			client.DeleteService(os.Args[3])
		case "deployment", "d":
			client.DeleteDeployment(os.Args[3])
		default:
			availableKinds()
		}
	case "apply":
		if len(os.Args) < 4 || os.Args[2] != "-f" {
			fmt.Println("expected command 'apply -f /path/to/config.yaml'")
			return
		}

		path := os.Args[3]
		client.Apply(path)
	case "restart":
		if len(os.Args) < 3 {
			needName()
			return
		}

		client.Restart(os.Args[2])
	case "help":
		printHelp()
	default:
		fmt.Printf("%s is not a valid command\n", args[1])
		printHelp()
	}
}

func availableKinds() {
	fmt.Println("Available kinds: deployment, service")
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
Server Commands:
	start [-d]              start Yetis server
	shutdown                terminate Yetis server
	info                    print server status
Resources Commands:
	apply -f FILENAME       apply a configuration from yaml file.
	get [-w] KIND           print a list the managed resources.
	logs [-f] NAME          print the logs of the deployment with NAME
	describe KIND NAME      print a detailed description of the selected resource
	delete KIND NAME        delete the resource, terminating its process
	restart NAME            restart the deployment according to its strategy type 
	help                    print the list of the commands
`)
}
