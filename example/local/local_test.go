package local

import (
	"os/exec"
	"testing"

	"github.com/dennwc/testproxy"
)

const (
	outFile = "./remote.test"
	testPkg = "../remote"
)

func TestLocal(t *testing.T) {
	buildRemote(t)
	bin := exec.Command(outFile)
	testproxy.RunTestBinary(t, bin)
}

func buildRemote(t testing.TB) {
	err := exec.Command("go", "test", "-c", "-o", outFile, testPkg).Run()
	if err != nil {
		t.Fatal(err)
	}
}
