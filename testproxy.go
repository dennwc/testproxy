// Package testproxy provides a set of tools to replay Go tests results from
// remote systems to the local Go test.
package testproxy

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

var debugFlag = os.Getenv("GO_TESTPROXY_DEBUG") == "true"

// Replay test events from JSON stream r on the current test suite.
func Replay(t *testing.T, r io.Reader) {
	t.Helper()
	var root testNode
	dec := json.NewDecoder(r)
	for {
		var e testEvent
		err := dec.Decode(&e)
		if err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		root.AddEvent(e)
	}
	root.RunTests(t)
}

// Runner represents a possibly remote go test.
type Runner interface {
	// RunAndWait runs a Go test in verbose mode and streams its stdout and stderr to provided writers.
	RunAndWait(stdout, stderr io.Writer) error
}

// Run the Go test using runner and return a JSON event stream.
func Run(t testing.TB, r Runner) io.ReadCloser {
	t.Helper()
	out := run(t, r, debugFlag)
	return ioutil.NopCloser(out)
}

// RunAndReplay runs the Go test using runner and replays all test results on t.
func RunAndReplay(t *testing.T, r Runner) {
	t.Helper()
	rc := Run(t, r)
	defer rc.Close()
	Replay(t, rc)
}

// RunTestBinary runs the compiled Go test binary and replays all test results on t.
func RunTestBinary(t *testing.T, bin *exec.Cmd) {
	t.Helper()
	r := NewTestBinary(bin)
	RunAndReplay(t, r)
}

// NewTestBinary creates a test runner from a given test binary.
func NewTestBinary(bin *exec.Cmd) Runner {
	verbose := false
	for _, a := range bin.Args {
		if a == "-test.v" {
			verbose = true
			break
		}
	}
	if !verbose {
		bin.Args = append(bin.Args, "-test.v")
	}
	return testBinary{cmd: bin}
}

type testBinary struct {
	cmd *exec.Cmd
}

func (r testBinary) RunAndWait(stdout, stderr io.Writer) error {
	r.cmd.Stdout = stdout
	r.cmd.Stderr = stderr
	err := r.cmd.Run()
	if _, ok := err.(*exec.ExitError); ok {
		err = nil
	}
	return err
}

func run(t testing.TB, bin Runner, debug bool) io.Reader {
	t.Helper()

	outBuf := bytes.NewBuffer(nil)
	errBuf := bytes.NewBuffer(nil)
	errc := make(chan error, 1)
	pr, pw := io.Pipe()
	go func() {
		defer pr.Close()
		cmd := exec.Command("go", "tool", "test2json")
		cmd.Stdout = outBuf
		cmd.Stdin = pr
		errc <- cmd.Run()
	}()

	outWriter := func(w io.Writer) io.Writer {
		return w
	}

	if debug {
		logf, err := os.Create("test.log")
		if err != nil {
			t.Fatal(err)
		}
		defer logf.Close()

		outWriter = func(w io.Writer) io.Writer {
			return io.MultiWriter(w, logf)
		}
	}

	err := bin.RunAndWait(outWriter(pw), outWriter(errBuf))
	if err != nil {
		t.Fatal(err)
	}

	pw.Close()
	if err != nil {
		t.Error(errBuf.String())
	}
	if err = <-errc; err != nil {
		t.Error(err)
	}
	return outBuf
}

type testEvent struct {
	Time    time.Time
	Action  string
	Package string
	Test    string
	Elapsed float64 // seconds
	Output  string
}

type testNode struct {
	Test   string
	Events []testEvent
	Sub    []*testNode
}

func (root *testNode) AddEvent(e testEvent) {
	if root.Test != "" {
		sub := strings.SplitN(e.Test, "/", 2)
		name := sub[0]
		if name != root.Test {
			panic(name + " != " + root.Test)
		}
		if len(sub) == 1 {
			root.Events = append(root.Events, e)
			return
		}
		e.Test = sub[1]
	} else if e.Test == "" {
		root.Events = append(root.Events, e)
		return
	}
	sub := strings.SplitN(e.Test, "/", 2)
	for _, s := range root.Sub {
		if s.Test == sub[0] {
			s.AddEvent(e)
			return
		}
	}
	s := &testNode{Test: sub[0]}
	root.Sub = append(root.Sub, s)
	s.AddEvent(e)
}

func (root *testNode) RunTests(t *testing.T) {
	started := false
replay:
	for _, e := range root.Events {
		switch e.Action {
		case "run":
			started = true
			for _, s := range root.Sub {
				s := s
				t.Run(s.Test, s.RunTests)
			}
		case "output":
			trimmed := strings.TrimSpace(e.Output)
			for _, pref := range []string{
				"=== RUN",
				"--- FAIL:",
				"--- PASS:",
			} {
				if strings.HasPrefix(trimmed, pref) {
					continue replay
				}
			}
			t.Log(strings.TrimSuffix(e.Output, "\n"))
		case "fail":
			t.Error()
		}
	}
	if !started {
		for _, s := range root.Sub {
			s := s
			t.Run(s.Test, s.RunTests)
		}
	}
}
