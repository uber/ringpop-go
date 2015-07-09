package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/tchannel/golang"
)

var numToStart = flag.Int("n", 0, "number of ringpops to start on execution")

var hostPortPattern = regexp.MustCompile(`^(\d+.\d+.\d+.\d+):\d+$`)
var logger = log.StandardLogger()
var hostsLock sync.Mutex
var channel, _ = tchannel.NewChannel("ringpop", nil)

// maps a ringpop hostport to a cmd which contains the process it runs in
var ringpops = make(map[string]*exec.Cmd)

// keeps track of which ringpops have been killed
var killed = make(map[string]bool)

// InvalidHostportError is an error when a string does not match hostPortPattern
type InvalidHostportError struct {
	s string
}

func (e InvalidHostportError) Error() string {
	return fmt.Sprintf("%s is not a valid hostport", e.s)
}

func makeCallNoArgs(call *tchannel.OutboundCall) error {
	var reqHeaders []byte
	if err := tchannel.NewArgWriter(call.Arg2Writer()).Write(reqHeaders); err != nil {
		return err
	}

	var reqBody []byte
	if err := tchannel.NewArgWriter(call.Arg3Writer()).Write(reqBody); err != nil {
		return err
	}

	var resHeaders []byte
	if err := tchannel.NewArgReader(call.Response().Arg2Reader()).Read(&resHeaders); err != nil {
		return err
	}

	var resBody []byte
	if err := tchannel.NewArgReader(call.Response().Arg3Reader()).Read(&resBody); err != nil {
		return err
	}

	return nil
}

func getRingpops(n int) []string {
	var hostports []string
	for hostport := range ringpops {
		hostports = append(hostports, hostport)
	}

	newHostports := make([]string, len(hostports), cap(hostports))
	newIndexes := rand.Perm(len(hostports))

	for o, n := range newIndexes {
		newHostports[n] = hostports[o]
	}

	if len(newHostports) < n {
		return newHostports
	}
	return newHostports[:n]
}

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
		return InvalidHostportError{hostport}
	}

	cmd := exec.Command("./testpop/testpop", "-p", hostport)
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

	// restore killed ringpops first
	for hostport := range killed {
		startRingpop(hostport)
		delete(killed, hostport)
		numStarted++
		if numStarted == n {
			return nil
		}
	}

	// spin up new ringpops if need be, starting from lowest available port
	for i := 1; numStarted != n; i++ {
		hostport := fmt.Sprintf("127.0.0.1:%v", 3000+i)
		if _, ok := ringpops[hostport]; ok {
			continue
		}
		time.Sleep(10 * time.Millisecond)
		err := startRingpop(hostport)
		if err != nil {
			log.WithFields(log.Fields{
				"error":    err,
				"hostport": hostport,
			}).Error("could not start ringpop")
			continue
		}
		numStarted++
	}

	return nil
}

func killRingpop(hostport string) error {
	if !hostPortPattern.Match([]byte(hostport)) {
		return InvalidHostportError{hostport}
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
	hostports := getRingpops(n)

	for _, hostport := range hostports {
		killRingpop(hostport)
	}

	return nil
}

func debugSet(hostport string) error {
	if !hostPortPattern.Match([]byte(hostport)) {
		return InvalidHostportError{hostport}
	}

	ctx, cancel := tchannel.NewContext(time.Second)
	defer cancel()

	call, err := channel.BeginCall(ctx, hostport, "ringpop", "/admin/debugSet", nil)
	if err != nil {
		return err
	}

	return makeCallNoArgs(call)
}

func debugClear(hostport string) error {
	if !hostPortPattern.Match([]byte(hostport)) {
		return InvalidHostportError{hostport}
	}

	ctx, cancel := tchannel.NewContext(time.Second)
	defer cancel()

	call, err := channel.BeginCall(ctx, hostport, "ringpop", "/admin/debugClear", nil)
	if err != nil {
		return err
	}

	return makeCallNoArgs(call)
}

func main() {
	logger.Formatter = &log.TextFormatter{}

	flag.Parse()

	startN(*numToStart)

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
		case "ds", "debugSet":
			if err := debugSet(opt); err != nil {
				fmt.Print(err)
			}

		case "dc", "debugClear":
			if err := debugClear(opt); err != nil {
				fmt.Print(err)
			}

		// start a ringpop on a specified hostport
		case "s", "start":
			err := startRingpop(opt)
			if err != nil {
				fmt.Printf("could not start ringpop on %s: %v\n", opt, err)
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
