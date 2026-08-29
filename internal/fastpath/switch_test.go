package fastpath_test

import (
	"context"
	"net"
	"sync"

	. "github.com/onsi/ginkgo/v2"

	"github.com/UnstoppableMango/terraform-provider-netgear/internal/fastpath"
)

// fakeSwitch is a scripted stand in for a FASTPATH CLI. It speaks whichever
// login flow it is configured for and answers commands from a lookup table.
type fakeSwitch struct {
	// model appears inside the prompt parentheses.
	model string

	// legacy embeds the user prompt in the interface configuration banner and
	// makes `enable` ask for a password, the way older firmware behaves.
	legacy bool

	// negotiate sends a telnet option request before the banner.
	negotiate bool

	// responses maps a command to the output the switch returns for it. Commands
	// absent from the map produce empty output.
	responses map[string]string

	// confirmSave makes a save ask for confirmation before writing NVRAM.
	confirmSave bool

	mu       sync.Mutex
	commands []string
}

// dialer returns a Dialer that serves a fresh CLI session per call, matching the
// way the client opens and closes a session around each operation. Sessions are
// torn down when the spec ends.
func (f *fakeSwitch) dialer() fastpath.Dialer {
	return func(ctx context.Context) (net.Conn, error) {
		client, server := net.Pipe()

		done := make(chan struct{})
		go func() {
			defer close(done)
			defer func() { _ = server.Close() }()
			f.serve(server)
		}()

		DeferCleanup(func() {
			_ = client.Close()
			<-done
		})

		return client, nil
	}
}

func (f *fakeSwitch) prompt(enabled bool) string {
	if enabled {
		return "\r\n(" + f.model + ") #"
	}
	return "\r\n(" + f.model + ") >"
}

func (f *fakeSwitch) received() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.commands...)
}

func (f *fakeSwitch) record(cmd string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.commands = append(f.commands, cmd)
}

// drain reads the connection continuously, the way a real switch does, and
// delivers complete lines on a buffered channel. Without this the fake would
// block writing while the client blocks writing its telnet replies, which a
// socket buffer would otherwise absorb. Telnet negotiation the client sends is
// dropped: it only ever emits three byte IAC sequences.
func drain(conn net.Conn) <-chan string {
	lines := make(chan string, 64)

	go func() {
		defer close(lines)

		var line []byte
		skip := 0
		buf := make([]byte, 1024)

		for {
			n, err := conn.Read(buf)
			for _, b := range buf[:n] {
				switch {
				case skip > 0:
					skip--
				case b == 255:
					skip = 2
				case b == '\n':
					lines <- string(line)
					line = nil
				case b != '\r':
					line = append(line, b)
				}
			}
			if err != nil {
				return
			}
		}
	}()

	return lines
}

func (f *fakeSwitch) serve(conn net.Conn) {
	lines := drain(conn)

	write := func(s string) bool {
		_, err := conn.Write([]byte(s))
		return err == nil
	}

	readLine := func() (string, bool) {
		line, ok := <-lines
		return line, ok
	}

	if f.negotiate {
		// IAC WILL SUPPRESS-GO-AHEAD, which the client is expected to refuse.
		if !write(string([]byte{255, 251, 3})) {
			return
		}
	}

	if f.legacy {
		if !write("\r\nApplying Interface configuration, please wait ...User:") {
			return
		}
	} else {
		if !write("\r\nUser:") {
			return
		}
	}

	if _, ok := readLine(); !ok {
		return
	}
	if !write("\r\nPassword:") {
		return
	}
	if _, ok := readLine(); !ok {
		return
	}
	if !write(f.prompt(false)) {
		return
	}

	enabled := false
	for {
		line, ok := readLine()
		if !ok {
			return
		}

		switch {
		case line == "enable" && !enabled:
			if f.legacy {
				if !write("\r\nPassword:") {
					return
				}
				if _, ok := readLine(); !ok {
					return
				}
			}
			enabled = true
			if !write(f.prompt(true)) {
				return
			}
		case line == "copy running-config startup-config" && f.confirmSave:
			f.record(line)
			if !write("\r\nAre you sure you want to save? (y/n)") {
				return
			}
			if _, ok := readLine(); !ok {
				return
			}
			if !write("\r\nConfiguration Saved!" + f.prompt(enabled)) {
				return
			}
		default:
			f.record(line)
			output := f.responses[line]
			if output != "" {
				output = "\r\n" + output
			}
			if !write(line + output + f.prompt(enabled)) {
				return
			}
		}
	}
}
