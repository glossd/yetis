package proxy

import (
	"fmt"
	"os/exec"
	"strings"
)

// no need for logPath
func Exec(port, targetPort int, logPath string) (int, int, error) {
	err := CreatePortForwarding(port, targetPort)
	// no longer need to return pid or updatePort
	return 0, 0, err
}

func CreatePortForwarding(fromPort, toPort int) error {
	// todo  return the line number?
	// https://askubuntu.com/a/579540/915003
	argStr := "-t nat -A OUTPUT " + portForwardRule(fromPort, toPort)
	cmd := exec.Command("iptables", strings.Split(argStr, " ")...)
	return cmd.Run()
}

func DeletePortForwarding(fromPort, toPort int) error {
	// https://stackoverflow.com/a/14521050/10160865
	line, err := getLine(fromPort, toPort)
	if err != nil {
		return err
	}
	argStr := fmt.Sprintf("-t nat -D OUTPUT %d", line)
	cmd := exec.Command("iptables", strings.Split(argStr, " ")...)
	return cmd.Run()
}

func UpdatePortForwarding(oldFromPort, newFromPort, toPort int) error {
	// https://stackoverflow.com/a/33468689/10160865
	line, err := getLine(oldFromPort, toPort)
	if err != nil {
		return err
	}
	argStr := fmt.Sprintf("-t nat -R OUTPUT %d ", line) + portForwardRule(oldFromPort, toPort)
	cmd := exec.Command("iptables", strings.Split(argStr, " ")...)
	return cmd.Run()
}

func getLine(fromPort, toPort int) (int, error) {

	return 0, fmt.Errorf("iptables rule not found")
}

func portForwardRule(fromPort, toPort int) string {
	return fmt.Sprintf("-o lo -p tcp --dport %d -j REDIRECT --to-port %d", fromPort, toPort)
}
