package bpftrace

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

type Process struct {
	cmd           *exec.Cmd
	StdoutScanner *bufio.Scanner
	NumProbes     int
}

func NewProcess(bpftracePath string, scriptPath string) (*Process, error) {
	cmd := exec.Command(bpftracePath, "-f", "json", scriptPath)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stdoutScanner := bufio.NewScanner(stdout)

	return &Process{
		cmd:           cmd,
		StdoutScanner: stdoutScanner,
	}, nil
}

func (p *Process) Start() error {
	log.Printf("starting `%s %s`...", p.cmd.Path, strings.Join(p.cmd.Args[1:], " "))
	err := p.cmd.Start()
	if err != nil {
		return err
	}

	go func() {
		err := p.cmd.Wait()
		if err != nil {
			log.Fatalf("bpftrace process exited: %v", err)
		}
		log.Printf("bpftrace process exited")
	}()

	// wait for bpftrace startup
	var out Output
Loop:
	for p.StdoutScanner.Scan() {
		line := p.StdoutScanner.Text()
		err := json.Unmarshal([]byte(line), &out)
		if err != nil {
			log.Printf("cannot parse JSON of line %v: %v", line, err)
			continue
		}

		switch out.Type {
		case "attached_probes":
			var probesData AttachedProbesData
			err := json.Unmarshal(out.Data, &probesData)
			if err != nil {
				return err
			}

			p.NumProbes = probesData.Probes
			break Loop
		default:
			log.Printf("unknown output type: %s", out.Type)
		}
	}

	if p.NumProbes > 0 {
		log.Printf("bpftrace started successfully (attached to %d probes)", p.NumProbes)
	}
	return nil
}

func (p *Process) SendSigusr1() error {
	return p.cmd.Process.Signal(syscall.SIGUSR1)
}

func (p *Process) SendSigint() error {
	return p.cmd.Process.Signal(syscall.SIGINT)
}
