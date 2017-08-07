package eows

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

type DoneChan struct {
	status int
	err    error
}

// cmdPumpStdin is in charge of receive characters and send them to stdin
func (e *ExecOverWS) cmdPumpStdin(inw *os.File) {

	done := make(chan DoneChan, 1)

	if e.InputEvent != "" && e.InputCB != nil {

		err := (*e.SocketIO).On(e.InputEvent, func(stdin string) {
			in, err := e.InputCB(e, string(stdin))
			if err != nil {
				e.logDebug("Error stdin: %s", err.Error())
				inw.Close()
				return
			}
			if _, err := inw.Write([]byte(in)); err != nil {
				e.logError("Error while writing to stdin: %s", err.Error())
			}
		})
		if err != nil {
			e.logError("Error stdin on event: %s", err.Error())
		}
	}

	// Monitor process exit
	go func() {
		status := 0
		sts, err := e.proc.Wait()
		if !sts.Success() {
			s := sts.Sys().(syscall.WaitStatus)
			status = s.ExitStatus()
		}
		done <- DoneChan{status, err}
	}()

	// Wait cmd complete
	select {
	case dC := <-done:
		e.ExitCB(e, dC.status, dC.err)
	case <-time.After(time.Duration(e.CmdExecTimeout) * time.Second):
		e.ExitCB(e, -999, fmt.Errorf("Exit Timeout for command ID %v", e.CmdID))
	}
}
