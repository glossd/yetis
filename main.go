package main

import (
	"fmt"
	"github.com/glossd/yetis/client"
	"github.com/glossd/yetis/common"
	"github.com/glossd/yetis/server"
	"log"
	"os"
	"os/user"
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
		if len(args) == 2 {
			server.Run("")
			return
		}

		if len(args) != 4 || args[2] != "-f" {
			printFlags("run", Flag{Def: "-f FILENAME", Des: "path to the yetis config"})
			return
		}

		server.Run(args[3])
	case "start":
		currentUser, err := user.Current()
		if err != nil {
			log.Fatalf("Unable to get current user: %s\n", err)
		}
		if currentUser.Username != "root" {
			log.Println("Warning: not running as root, Yetis won't be to create a proxy")
		}
		if len(args) == 2 {
			client.StartBackground("")
			return
		}
		if len(args) != 4 || args[2] != "-f" {
			printFlags("start", Flag{Def: "-f FILENAME", Des: "path to the yetis config"})
			return
		}

		client.StartBackground(args[3])
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
	case "get": // deprecated.
		fallthrough
	case "list": // is back in business
		if len(args) == 2 {
			client.GetDeployments()
			return
		}
		if args[2] == "-w" {
			client.WatchGetDeployments()
		} else {
			printHelp()
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
		if len(os.Args) < 3 {
			needName()
			return
		}
		client.DescribeDeployment(os.Args[2])
	case "delete":
		if len(os.Args) < 3 {
			needName()
			return
		}
		client.DeleteDeployment(os.Args[2])
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

type Flag struct {
	// Definition e.g. -f FILENAME
	Def string
	// Description e.g. path to the config
	Des string
}

func printFlags(cmd string, flags ...Flag) {
	fmt.Printf("The flags for %s command are:\n", cmd)
	for _, f := range flags {
		fmt.Println("	" + f.Def + "    " + f.Des)
	}
}

func needName() {
	fmt.Println("provide the name of the deployment")
}

func printHelp() {
	fmt.Printf(`The commands are:
Server Commands:
	start [-f FILENAME]     start Yetis server
	shutdown                terminate Yetis server
	info                    print server status
Resources Commands:
	apply -f FILENAME       apply a configuration from yaml file. Creates new deployments or restarts existing ones
	list [-w]               print a list the managed deployment
	logs [-f] NAME          print the logs of the deployment with NAME
	describe NAME           print a detailed description of the selected deployment
	delete NAME             delete the deployment, terminating its process
	restart NAME            restart the deployment according to its strategy type 
	help                    print the list of the commands
`)
}
