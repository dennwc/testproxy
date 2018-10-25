package remote

import "testing"

func TestRemoteSuccess(t *testing.T) {
	t.Log("success!")
}

func TestRemoteFailure(t *testing.T) {
	t.Fatal("error!")
}
