package cmdutils

import (
	"log"
	"os"
	"os/exec"
	"runtime"
	"sync"
)

type CLI uint8

const (
	CLI_AUTO CLI = iota
	CLI_POWERSHELL
	CLI_CMD
	CLI_BASH
)

type Executer interface {
	// Execute command with selected executer
	Execute(command string, flags ...string) (string, error)

	// Execute commands with selected executer and get results asynchronously
	AsyncExecute(command string, flags ...string) (chan OutputMessage, error)

	// Clear console output
	Clear()

	// Make directory with given name and permissions
	//
	// Permission is 755 by default, but you can change permissions with arguments like this:
	// perm[0] for owner, perm[1] for group and perm[2] for others.
	Mkdir(name string, perm ...PermissionMode) error

	// Make directory with given name and permissions, then switch working directory to created
	MkdirAndCd(name string, perm ...PermissionMode) error

	// Change working directory to given path
	Cd(path string) error

	// Remove file or directory of given path
	Rm(path string) error

	// Debug mode
	Debug()
}

type executer struct {
	cliExecuter string
	cliParams   string
	isDebug     bool
}

func NewExecuter(cli CLI) Executer {
	var cliExecuter, cliParams string

	switch cli {
	case CLI_POWERSHELL:
		{
			cliExecuter = "powershell.exe"
			cliParams = "/c"
		}

	case CLI_CMD:
		{
			cliExecuter = "cmd.exe"
			cliParams = "/c"
		}

	case CLI_BASH:
		{
			cliExecuter = "/bin/bash"
			cliParams = "-c"
		}

	default:
		{
			if runtime.GOOS == "windows" {
				cliExecuter = "powershell.exe"
				cliParams = "/c"
			} else {
				cliExecuter = "/bin/bash"
				cliParams = "-c"
			}
		}
	}

	return &executer{
		cliExecuter: cliExecuter,
		cliParams:   cliParams,
	}
}

// Execute command with selected executer
func (e *executer) Execute(command string, flags ...string) (string, error) {
	cmd := exec.Command(e.cliExecuter, e.cliParams, command)
	cmd.Args = append(cmd.Args, flags...)

	if e.isDebug {
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
	}

	output, err := cmd.Output()
	if err != nil {
		log.Printf("Couldn't Run Command << %s >>\nerror: %s", command, err.Error())
	}

	return string(output), err
}

// Set to Debug mode
func (e *executer) Debug() {
	e.isDebug = true
}

// Clear console output
func (e *executer) Clear() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}

	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		log.Fatal("Couldn't Clear Terminal: ", err)
	}
}

// Make directory with given name and permissions
//
// Permission is 755 by default, but you can change permissions with arguments like this:
// perm[0] for owner, perm[1] for group and perm[2] for others.
func (e *executer) Mkdir(name string, perm ...PermissionMode) error {
	p := [3]PermissionMode{Perm_rwx, Perm_rox, Perm_rox}

	for i, val := range perm {
		if i < len(p) {
			p[i] = val
		}
	}

	permBits, err := mergePerm(p[0], p[1], p[2])
	if err != nil {
		return err
	}

	return os.Mkdir(name, os.FileMode(permBits))
}

// Make directory with given name and permissions, then switch working directory to created
func (e *executer) MkdirAndCd(name string, perm ...PermissionMode) error {
	if err := e.Mkdir(name, perm...); err != nil {
		return err
	}

	return os.Chdir(name)
}

// Change working directory to given path
func (e *executer) Cd(path string) error {
	return os.Chdir(path)
}

// Remove file or directory of given path
func (e *executer) Rm(path string) error {
	return os.Remove(path)
}

type OutputMessage struct {
	Line     string
	Error    error
	IsStderr bool
}

// Execute commands with selected executer and get results asynchronously
func (e *executer) AsyncExecute(command string, flags ...string) (chan OutputMessage, error) {
	cmd := exec.Command(e.cliExecuter, e.cliParams, command)
	cmd.Args = append(cmd.Args, flags...)

	if e.isDebug {
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	stderrBuffer := make([]byte, 1)
	stdoutBuffer := make([]byte, 1)
	output := make(chan OutputMessage, 10)

	var wg sync.WaitGroup

	wg.Add(1)
	// pipe for stderr
	go func() {
	console:
		for {
			var line string
		line:
			for {
				_, err := stderr.Read(stderrBuffer)
				if err != nil {
					output <- OutputMessage{Error: err}

					break console
				}

				if string(stderrBuffer) == "\n" {
					break line
				}

				line += string(stderrBuffer)
			}

			output <- OutputMessage{Line: line, IsStderr: true}

			line = ""
		}

		wg.Done()
	}()

	wg.Add(1)
	// pipe for stdout
	go func() {
	console:
		for {
			var line string
		line:
			for {
				_, err := stdout.Read(stdoutBuffer)
				if err != nil {
					output <- OutputMessage{Error: err}

					break console
				}

				if string(stdoutBuffer) == "\n" {
					break line
				}

				line += string(stdoutBuffer)
			}

			output <- OutputMessage{Line: line}

			line = ""
		}

		wg.Done()
	}()

	// pipe for wait and close
	go func() {
		err := cmd.Wait()
		if err != nil {
			output <- OutputMessage{Error: err}
		}

		wg.Wait()

		close(output)
	}()

	return output, nil
}
