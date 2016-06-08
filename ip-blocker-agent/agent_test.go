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
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

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

var agentExe string

func TestMain(m *testing.M) {
	dir, err := ioutil.TempDir("", "go-test-agent")
	if err != nil {
		panic(err)
	}

	agentExe = dir + "/ip-blocker-agent"

	cmd := exec.Command("go", "build", "-o", agentExe, ".")
	cmd.Stderr = os.Stderr
	cmd.Run()

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type quitReader struct {
	ch chan struct{}
}

func (r quitReader) Read(p []byte) (n int, err error) {
	<-r.ch

	const quit = "q\n"
	copy(p, quit)
	return len(quit), io.EOF
}

func TestTUI(t *testing.T) {
	name := fmt.Sprintf("/go-test-%d", nameRand.Int())

	cmd := exec.Command(agentExe, "-name", name)

	quit := quitReader{make(chan struct{})}
	cmd.Stdin = io.MultiReader(strings.NewReader(`+192.0.2.0
+192.0.2.1
-192.0.2.1
+192.0.2.128/31
+2001:db8::
+2001:db8::1
!
b
+192.0.2.0/29
+2001:db8::/126
-192.0.2.3
-192.0.2.4
B
`), quit)

	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	go func() {
		defer close(quit.ch)

		time.Sleep(100 * time.Millisecond)

		client, err := blocker.Open(name)
		if err != nil {
			t.Error(err)
			return
		}

		for _, addr := range [...]string{
			"192.0.2.0", "192.0.2.1", "192.0.2.2", "192.0.2.5", "192.0.2.6", "192.0.2.7",
			"2001:db8::", "2001:db8::1", "2001:db8::2", "2001:db8::3",
		} {
			has, err := client.Contains(net.ParseIP(addr))
			if err != nil {
				t.Error(err)
			}

			if !has {
				t.Errorf("server does not contain %s", addr)
			}
		}
	}()

	cmd.Run()

	if stderr.Len() != 0 {
		t.Errorf("stderr was not empty, got: %s", stderr.Bytes())
	}

	expect := `IP4: 0, IP6: 0, IP6 routes: 0
IP4: 1, IP6: 0, IP6 routes: 0
IP4: 2, IP6: 0, IP6 routes: 0
IP4: 1, IP6: 0, IP6 routes: 0
IP4: 3, IP6: 0, IP6 routes: 0
IP4: 3, IP6: 1, IP6 routes: 0
IP4: 3, IP6: 2, IP6 routes: 0
IP4: 0, IP6: 0, IP6 routes: 0
IP4: 6, IP6: 4, IP6 routes: 0
`
	if stdout.String() != expect {
		t.Error("stdout was invalid")
		t.Errorf("expected:\t%q", expect)
		t.Errorf("got:\t\t%q", stdout.String())
	}
}
