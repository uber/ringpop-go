package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
)

var numToStart = flag.Int("start", 0, "number of ringpops to start on execution")

var hostPortPattern = regexp.MustCompile(`^(\d+.\d+.\d+.\d+):\d+$`)
var logger = log.StandardLogger()
var hostsLock sync.Mutex

// maps a ringpop hostport to a cmd which contains the process it runs in
var ringpops = make(map[string]*exec.Cmd)
var killed = make(map[string]bool)

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
	if !hostPortPattern.Match([]byte(hostport)) {
		return errors.New("not a valid hostport pattern")
	}

	cmd := exec.Command("./testpop/testpop", "-hostport", hostport)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := writeHostsFile(hostport); err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	ringpops[hostport] = cmd
	fmt.Printf("starting ringpop on %s\n", hostport)
	return nil
}

func startN(n int) error {
	numStarted := 0

	for hostport := range killed {
		startRingpop(hostport)
		numStarted++
		if numStarted == n {
			return nil
		}
	}

	for i := 1; i <= n; i++ {
		hostport := fmt.Sprintf("127.0.0.1:%v", 3000+i)
		time.Sleep(10 * time.Millisecond)
		err := startRingpop(hostport)
		if err != nil {
			log.WithFields(log.Fields{
				"error":    err,
				"hostport": hostport,
			}).Error("could not start ringpop")
			continue
		}
	}

	return nil
}

func killRingpop(hostport string) error {
	if !hostPortPattern.Match([]byte(hostport)) {
		return fmt.Errorf("%s is not a valid hostport", hostport)
	}

	cmd, ok := ringpops[hostport]
	if !ok {
		return fmt.Errorf("no ringpop exists on %s", hostport)
	}

	if err := cmd.Process.Kill(); err != nil {
		return err
	}

	delete(ringpops, hostport)
	killed[hostport] = true

	if err := writeHostsFile(""); err != nil {
		return err
	}

	fmt.Printf("killed ringpop at %s successfully\n", hostport)
	return nil
}

func killN(n int) error {
	println(n)
	return nil
}

func main() {
	logger.Formatter = &log.TextFormatter{}

	flag.Parse()

	for i := 1; i <= *numToStart; i++ {
		hostport := fmt.Sprintf("127.0.0.1:%v", 3000+i)
		time.Sleep(10 * time.Millisecond)
		err := startRingpop(hostport)
		if err != nil {
			log.WithFields(log.Fields{
				"error":    err,
				"hostport": hostport,
			}).Error("could not start ringpop")
			continue
		}
	}

	var input, opt string

INPUT:
	for {
		n, err := fmt.Scanln(&input, &opt)
		if err != nil {
			if n < 1 {
				println(err)
				continue
			}
		}

		switch input {
		// start a ringpop on a specified hostport
		case "s", "start":
			err := startRingpop(opt)
			if err != nil {
				fmt.Printf("could not start ringpop on %s: %v\n", opt, err)
				continue
			}

		// start n ringpops sub-processes, prioritized those that were previously killed
		case "sn", "startn":
			n, err := strconv.Atoi(opt)
			if err != nil {
				println("expected int as argument to startn")
				continue
			}
			startN(n)

		// kill a ringpop at a specified hostport
		case "k", "kill":
			err := killRingpop(opt)
			if err != nil {
				fmt.Printf("could not kill ringpop at %s: %v\n", opt, err)
			}

		// kill n ringpop sub-processes
		case "kn", "killn":
			n, err := strconv.Atoi(opt)
			if err != nil {
				println("expected int as argument to startn")
				continue
			}
			killN(n)

		// quit the program and kill all sub-processes
		case "\u0003", "q", "quit":
			break INPUT

		default:
			continue
		}
	}

	errs := quit()
	for _, err := range errs {
		println(err)
	}
}
