package eows

import (
	"bufio"
	"io"
	"strings"
)

// scanBlocks - gain character by character (or as soon as one or more characters are available)
func scanBlocks(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	return len(data), data, nil
}

// cmdPumpStdout is in charge to forward stdout in websocket
func (e *ExecOverWS) cmdPumpStdout(r io.Reader, done chan struct{}) {

	defer func() {
	}()

	sc := bufio.NewScanner(r)

	// else use default sc.ScanLines
	if e.OutSplit == SplitChar {
		sc.Split(scanBlocks)
	}

	for sc.Scan() {
		e.OutputCB(e, sc.Text(), "")
	}
	if sc.Err() != nil && !strings.Contains(sc.Err().Error(), "file already closed") {
		e.logError("stdout scan: %v", sc.Err())
	}

	close(done)
}

// cmdPumpStderr is in charge to forward stderr in websocket
func (e *ExecOverWS) cmdPumpStderr(r io.Reader) {

	defer func() {
	}()
	sc := bufio.NewScanner(r)

	// else use default sc.ScanLines
	if e.OutSplit == SplitChar {
		sc.Split(scanBlocks)
	}

	for sc.Scan() {
		e.OutputCB(e, "", sc.Text())
	}
	if sc.Err() != nil && !strings.Contains(sc.Err().Error(), "file already closed") {
		e.logError("stderr scan: %v", sc.Err())
	}
}
