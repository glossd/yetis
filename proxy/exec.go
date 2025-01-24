package proxy

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

var ErrRuleNotFound = fmt.Errorf("iptables rule not found")

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

func UpdatePortForwarding(fromPort, oldToPort, newToPort int) error {
	// https://stackoverflow.com/a/33468689/10160865
	if oldToPort == newToPort {
		return nil
	}
	line, err := getLine(fromPort, oldToPort)
	if err != nil {
		if err == ErrRuleNotFound {
			err := CreatePortForwarding(fromPort, newToPort)
			if err != nil {
				return fmt.Errorf("rule was not found and failed to create it: %s", err)
			}
			return nil
		}
		return err
	}
	argStr := fmt.Sprintf("-t nat -R OUTPUT %d ", line) + portForwardRule(fromPort, newToPort)
	cmd := exec.Command("iptables", strings.Split(argStr, " ")...)
	return cmd.Run()
}

func getLine(fromPort, toPort int) (int, error) {
	output, err := exec.Command("iptables -t nat -L OUTPUT --line-numbers").Output()
	if err != nil {
		return 0, fmt.Errorf("failed to list iptables rules: %s", err)
	}
	return extractLine(string(output), fromPort, toPort)
}

func extractLine(output string, fromPort, toPort int) (int, error) {
	lines := strings.Split(output, "\n")
	fromPortStr := strconv.Itoa(fromPort)
	// for some reason iptables list replaces the well-known ports with their names.
	switch fromPort {
	case 80:
		fromPortStr = "http"
	case 443:
		fromPortStr = "https"
	}

	for _, line := range lines {
		if strings.Contains(line, fromPortStr) && strings.Contains(line, strconv.Itoa(toPort)) {
			lexes := strings.Split(line, " ")
			num, err := strconv.Atoi(lexes[0])
			if err != nil {
				return 0, fmt.Errorf("failed to extract line number from '%s'", line)
			}
			return num, nil
		}
	}
	return 0, ErrRuleNotFound
}

func portForwardRule(fromPort, toPort int) string {
	return fmt.Sprintf("-o lo -p tcp --dport %d -j REDIRECT --to-port %d", fromPort, toPort)
}
