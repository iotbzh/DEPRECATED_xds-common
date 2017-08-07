// Package eows is used to Execute commands Over WebSocket
package eows

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/googollee/go-socket.io"
)

// OnInputCB is the function callback used to receive data
type OnInputCB func(e *ExecOverWS, stdin string) (string, error)

// EmitOutputCB is the function callback used to emit data
type EmitOutputCB func(e *ExecOverWS, stdout, stderr string)

// EmitExitCB is the function callback used to emit exit proc code
type EmitExitCB func(e *ExecOverWS, code int, err error)

// Inspired by :
// https://github.com/gorilla/websocket/blob/master/examples/command/main.go

// ExecOverWS .
type ExecOverWS struct {
	Cmd      string           // command name to execute
	Args     []string         // command arguments
	SocketIO *socketio.Socket // websocket
	Sid      string           // websocket ID
	CmdID    string           // command ID

	// Optional fields
	Env            []string                // command environment variables
	CmdExecTimeout int                     // command execution time timeout
	Log            *logrus.Logger          // logger (nil if disabled)
	InputEvent     string                  // websocket input event name
	InputCB        OnInputCB               // stdin callback
	OutputCB       EmitOutputCB            // stdout/stderr callback
	ExitCB         EmitExitCB              // exit proc callback
	UserData       *map[string]interface{} // user data passed to callbacks

	// Private fields
	proc *os.Process
}

var cmdIDMap = make(map[string]*ExecOverWS)

// New creates a new instace of eows
func New(cmd string, args []string, so *socketio.Socket, soID, cmdID string) *ExecOverWS {

	e := &ExecOverWS{
		Cmd:            cmd,
		Args:           args,
		SocketIO:       so,
		Sid:            soID,
		CmdID:          cmdID,
		CmdExecTimeout: -1, // default no timeout
	}

	cmdIDMap[cmdID] = e

	return e
}

// GetEows gets ExecOverWS object from command ID
func GetEows(cmdID string) *ExecOverWS {
	if _, ok := cmdIDMap[cmdID]; !ok {
		return nil
	}
	return cmdIDMap[cmdID]
}

// Start executes the command and redirect stdout/stderr into a WebSocket
func (e *ExecOverWS) Start() error {
	var err error
	var outr, outw, errr, errw, inr, inw *os.File

	bashArgs := []string{"/bin/bash", "-c", e.Cmd + " " + strings.Join(e.Args, " ")}

	// no timeout == 1 year
	if e.CmdExecTimeout == -1 {
		e.CmdExecTimeout = 365 * 24 * 60 * 60
	}

	// Create pipes
	outr, outw, err = os.Pipe()
	if err != nil {
		err = fmt.Errorf("Pipe stdout error: " + err.Error())
		goto exitErr
	}

	errr, errw, err = os.Pipe()
	if err != nil {
		err = fmt.Errorf("Pipe stderr error: " + err.Error())
		goto exitErr
	}

	inr, inw, err = os.Pipe()
	if err != nil {
		err = fmt.Errorf("Pipe stdin error: " + err.Error())
		goto exitErr
	}

	e.proc, err = os.StartProcess("/bin/bash", bashArgs, &os.ProcAttr{
		Files: []*os.File{inr, outw, errw},
		Env:   append(os.Environ(), e.Env...),
	})
	if err != nil {
		err = fmt.Errorf("Process start error: " + err.Error())
		goto exitErr
	}

	go func() {
		defer outr.Close()
		defer outw.Close()
		defer errr.Close()
		defer errw.Close()
		defer inr.Close()
		defer inw.Close()

		stdoutDone := make(chan struct{})
		go e.cmdPumpStdout(outr, stdoutDone)
		go e.cmdPumpStderr(errr)

		// Blocking function that poll input or wait for end of process
		e.cmdPumpStdin(inw)

		// Some commands will exit when stdin is closed.
		inw.Close()

		defer outr.Close()

		if status, err := e.proc.Wait(); err == nil {
			// Other commands need a bonk on the head.
			if !status.Exited() {
				if err := e.proc.Signal(os.Interrupt); err != nil {
					e.logError("Proc interrupt:", err)
				}

				select {
				case <-stdoutDone:
				case <-time.After(time.Second):
					// A bigger bonk on the head.
					if err := e.proc.Signal(os.Kill); err != nil {
						e.logError("Proc term:", err)
					}
					<-stdoutDone
				}
			}
		}

		delete(cmdIDMap, e.CmdID)
	}()

	return nil

exitErr:
	for _, pf := range []*os.File{outr, outw, errr, errw, inr, inw} {
		pf.Close()
	}
	return err
}

func (e *ExecOverWS) logDebug(format string, a ...interface{}) {
	if e.Log != nil {
		e.Log.Debugf(format, a)
	}
}

func (e *ExecOverWS) logError(format string, a ...interface{}) {
	if e.Log != nil {
		e.Log.Errorf(format, a)
	}
}
