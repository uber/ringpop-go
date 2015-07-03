package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
)

var logger = log.StandardLogger()
var hostsLock sync.Mutex

// maps a ringpop hostport to a cmd which contains the process it runs in
var ringpops = make(map[string]*exec.Cmd)

func quit() (errs []error) {
	for hostport := range ringpops {
		if err := killRingpop(hostport); err != nil {
			errs = append(errs, err)
			continue
		}
	}

	return errs
}

// TODO: make this less shitty
func writeHostsFile(newHostport string) error {
	os.Remove("./testpop/hosts.json")
	file, err := os.Create("./testpop/hosts.json")
	if err != nil {
		return err
	}

	file.WriteString("[\n")

	for hostport := range ringpops {
		file.WriteString(fmt.Sprintf("\t\"%s\",\n", hostport))
	}
	if newHostport != "" {
		file.WriteString(fmt.Sprintf("\t\"%s\"\n", newHostport))
	}

	file.WriteString("]\n")

	return nil
}

func startRingpop(hostport string) error {
	cmd := exec.Command("./testpop/testpop", hostport)
	// cmd := exec.Command("./mock_testpop/mock_testpop", hostport)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := writeHostsFile(hostport); err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	ringpops[hostport] = cmd
	return nil
}

func killRingpop(hostport string) error {
	cmd, ok := ringpops[hostport]
	if !ok {
		return errors.New("no ringpop on specified hostport")
	}

	if err := cmd.Process.Kill(); err != nil {
		return err
	}

	delete(ringpops, hostport)
	println("boom!", hostport)

	if err := writeHostsFile(""); err != nil {
		return err
	}

	return nil
}

func main() {
	logger.Formatter = &log.TextFormatter{}

	args := os.Args
	if len(args) != 2 {
		log.Fatal("expected single argument")
	}

	numRingpops, err := strconv.Atoi(args[1])
	if err != nil {
		log.Fatal("expected integer argument")
	}

	for i := 1; i <= numRingpops; i++ {
		hostport := fmt.Sprintf("127.0.0.1:%v", 3000+i)

		time.Sleep(500 * time.Millisecond)

		err := startRingpop(hostport)
		if err != nil {
			log.WithFields(log.Fields{
				"error":    err,
				"hostport": hostport,
			}).Error("could not start ringpop")
			continue
		}
	}

	var input string
INPUT:
	for {
		_, err := fmt.Scan(&input)
		if err != nil {
			println(err)
			continue
		}

		switch input {
		case "q":
			break INPUT
		}
	}

	errs := quit()
	for _, err := range errs {
		println(err)
	}
}
