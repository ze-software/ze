// Design: docs/contributing/terminal-demos.md -- the lab a user would build
// Related: scenarios_showcase.go -- runShowcaseLab, the lab this checks

package siteterminaldemo

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"
)

// validateShowcaseLab starts the base lab and checks what every topic
// recording relies on before it loads its own sections: the transit and frr
// sessions are established on the base config alone, and `announce` brings
// the customer session up as the third.
func validateShowcaseLab() error {
	defer func() { _ = runShowcaseLab(commandStop, nil, io.Discard) }() //nolint:errcheck // the stop's report line is the only thing it can fail to write
	if err := runShowcaseLab(actionPrepare, nil, io.Discard); err != nil {
		return err
	}
	if err := runShowcaseLab(commandStart, nil, io.Discard); err != nil {
		return err
	}
	var peers bytes.Buffer
	if err := runShowcaseLab(commandCLI, []string{showPeerListRaw}, &peers); err != nil {
		return err
	}
	if err := requireAll(peers.String(), `"name": "transit"`, `"name": "frr"`); err != nil {
		return err
	}
	if err := runShowcaseLab(showcaseAnnounce, nil, io.Discard); err != nil {
		return err
	}
	var text string
	for range 300 {
		peers.Reset()
		if err := runShowcaseLab(commandCLI, []string{showPeerListRaw}, &peers); err != nil {
			return err
		}
		text = peers.String()
		if strings.Count(text, `"state": "established"`) == showcaseEstablished+1 {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("validation failed: the customer session did not join transit and frr as established after announce\n%s", text)
}
