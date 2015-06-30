package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	log "github.com/Sirupsen/logrus"
)

var hostsFile = "./ringpop_tester/hosts.json"

// kills all spawned processes
func quit(cmds []*exec.Cmd) []error {
	for _, cmd := range cmds {
		if cmd.Process == nil {
			continue
		}

		pid := cmd.Process.Pid

		killCmd := exec.Command("kill", strconv.Itoa(pid))
		err := killCmd.Run()
		if err != nil {
			// could not kill process ahh
		}
		println(pid)
	}

	return nil
}

func main() {
	args := os.Args
	if len(args) != 2 {
		log.Fatal("too many/not enough arguments")
	}

	numRingpops, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatal("expected number argument")
	}

	var cmds []*exec.Cmd

	var hosts []string

	// spin up ringpops
	for i := 1; i <= numRingpops; i++ {
		port := 3000 + i
		hostport := fmt.Sprintf("127.0.0.1:%v", port)

		hosts = append(hosts, hostport)

		// add host to hosts file
		// ...

		cmd := exec.Command("./test/ringpop_tester", hostport, "&")
		cmds = append(cmds, cmd)

		err := cmd.Run()
		if err != nil {
			continue
		}
	}

	var input string

run:
	for _, err := fmt.Scanf("%s", &input); err == nil; {
		switch input {
		case "k", "kill":
			print(input)
		case "q", "quit":
			quit(cmds)
			break run
		default:
			// do nothing
		}
	}
}
