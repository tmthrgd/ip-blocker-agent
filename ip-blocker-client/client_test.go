// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package main

import (
	"bytes"
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"

	"github.com/tmthrgd/ip-blocker-agent"
)

var nameRand *rand.Rand

func init() {
	var seed [8]byte

	if _, err := crand.Read(seed[:]); err != nil {
		panic(err)
	}

	seedInt := int64(binary.LittleEndian.Uint64(seed[:]))
	nameRand = rand.New(rand.NewSource(seedInt))
}

var clientExe string

func TestMain(m *testing.M) {
	dir, err := ioutil.TempDir("", "go-test-client")
	if err != nil {
		panic(err)
	}

	clientExe = dir + "/ip-blocker-client"

	cmd := exec.Command("go", "build", "-o", clientExe, ".")
	cmd.Stderr = os.Stderr
	cmd.Run()

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func setup() (*blocker.Server, error) {
	name := fmt.Sprintf("/go-test-%d", nameRand.Int())

	return blocker.New(name, 0600)
}

func TestQuery(t *testing.T) {
	server, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	if err := server.Insert(net.ParseIP("192.0.2.0")); err != nil {
		t.Fatal(err)
	}

	if err := server.Insert(net.ParseIP("2001:db8::")); err != nil {
		t.Fatal(err)
	}

	for _, addr := range [...]string{"192.0.2.0", "2001:db8::"} {
		err = exec.Command(clientExe, "-name", server.Name(), addr).Run()
		if exit, ok := err.(*exec.ExitError); ok {
			if status, ok := exit.Sys().(syscall.WaitStatus); ok {
				t.Errorf("%s did not exit with code 0, got %d", clientExe, status.ExitStatus())
			} else {
				t.Log("cannot get exit code")
				t.Error(err)
			}
		} else if err != nil {
			t.Error(err)
		}
	}

	for _, addr := range [...]string{"192.0.2.1", "2001:db8::1"} {
		err = exec.Command(clientExe, "-name", server.Name(), addr).Run()
		if exit, ok := err.(*exec.ExitError); ok {
			if status, ok := exit.Sys().(syscall.WaitStatus); ok {
				if status.ExitStatus() != 1 {
					t.Errorf("%s did not exit with code 1, got %d", clientExe, status.ExitStatus())
				}
			} else {
				t.Log("cannot get exit code")
			}
		} else if err != nil {
			t.Error(err)
		} else {
			t.Errorf("%s did not exit with code 1", clientExe)
		}
	}
}

func TestTUI(t *testing.T) {
	server, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	if err := server.Insert(net.ParseIP("192.0.2.0")); err != nil {
		t.Fatal(err)
	}

	if err := server.Insert(net.ParseIP("2001:db8::")); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(clientExe, "-name", server.Name())
	cmd.Stdin = strings.NewReader(`192.0.2.0
192.0.2.1
192.0.2.2
192.0.2.0
2001:db8::
2001:db8::1
?
q`)
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Run()

	if stderr.Len() != 0 {
		t.Errorf("stderr was not empty, got: %s", stderr.Bytes())
	}

	expect := `IP4: 1, IP6: 1, IP6 routes: 0
true
false
false
true
true
false
IP4: 1, IP6: 1, IP6 routes: 0
`
	if stdout.String() != expect {
		t.Error("stdout was invalid")
		t.Errorf("expected:\t%q", expect)
		t.Errorf("got:\t%q", stdout.String())
	}
}
