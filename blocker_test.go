// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

// Note: you can cleanup any stray shared memory with:
// 	find /dev/shm/ -name "go-test-*" -exec unlink {} \;

import (
	"bytes"
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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

func setup(withClient bool) (*Server, *Client, error) {
	name := fmt.Sprintf("/go-test-%d", nameRand.Int())

	server, err := New(name, 0600)
	if err != nil {
		return nil, nil, err
	}

	if !withClient {
		return server, nil, nil
	}

	client, err := Open(name)
	if err != nil {
		server.Close()
		server.Unlink()

		return nil, nil, err
	}

	return server, client, nil
}

func testAddress(t *testing.T, addrs ...string) {
	server, client, err := setup(true)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()
	defer client.Close()

	for _, addr := range addrs {
		has, err := client.Contains(net.ParseIP(addr))
		if err != nil {
			t.Error(err)
		}

		if has {
			t.Error("blocklist contains entry before any added")
		}
	}

	if err = server.Batch(); err != nil {
		t.Fatal(err)
	}

	for _, addr := range addrs {
		if err = server.Insert(net.ParseIP(addr)); err != nil {
			t.Error(err)
		}
	}

	if err = server.Commit(); err != nil {
		t.Fatal(err)
	}

	for _, addr := range addrs {
		has, err := client.Contains(net.ParseIP(addr))
		if err != nil {
			t.Error(err)
		}

		if !has {
			t.Error("blocklist does not contain entry after insert")
		}
	}

	if err = server.Batch(); err != nil {
		t.Fatal(err)
	}

	for _, addr := range addrs {
		if err = server.Remove(net.ParseIP(addr)); err != nil {
			t.Error(err)
		}
	}

	if err = server.Commit(); err != nil {
		t.Fatal(err)
	}

	for _, addr := range addrs {
		has, err := client.Contains(net.ParseIP(addr))
		if err != nil {
			t.Error(err)
		}

		if has {
			t.Error("blocklist contains entry after remove")
		}
	}
}

func TestIP4(t *testing.T) {
	testAddress(t, "192.0.2.0", "192.0.2.1", "198.51.100.0", "198.51.100.1", "203.0.113.0", "203.0.113.1")
}

func TestIP6(t *testing.T) {
	testAddress(t, "2001:db8::", "2001:db8::1", "2001:db8::f", "2001:db8::dead:beef")
}

func TestMixed(t *testing.T) {
	testAddress(t, "192.0.2.0", "192.0.2.1", "198.51.100.0", "198.51.100.1", "203.0.113.0", "203.0.113.1", "2001:db8::", "2001:db8::1", "2001:db8::f", "2001:db8::dead:beef")
}

func testRange(t *testing.T, ipranges []string, addrs ...string) {
	server, client, err := setup(true)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()
	defer client.Close()

	for _, addr := range addrs {
		has, err := client.Contains(net.ParseIP(addr))
		if err != nil {
			t.Error(err)
		}

		if has {
			t.Error("blocklist contains entry before any added")
		}
	}

	if err = server.Batch(); err != nil {
		t.Fatal(err)
	}

	for _, iprange := range ipranges {
		ip, ipnet, err := net.ParseCIDR(iprange)
		if err != nil {
			panic(err)
		}

		if err = server.InsertRange(ip, ipnet); err != nil {
			t.Fatal(err)
		}
	}

	if err = server.Commit(); err != nil {
		t.Fatal(err)
	}

	for _, addr := range addrs {
		has, err := client.Contains(net.ParseIP(addr))
		if err != nil {
			t.Error(err)
			continue
		}

		if !has {
			t.Errorf("blocklist does not contain entry after insert: %v, %s", ipranges, addr)
		}
	}

	if err = server.Batch(); err != nil {
		t.Fatal(err)
	}

	for _, iprange := range ipranges {
		ip, ipnet, err := net.ParseCIDR(iprange)
		if err != nil {
			panic(err)
		}

		if err = server.RemoveRange(ip, ipnet); err != nil {
			t.Fatal(err)
		}
	}

	if err = server.Commit(); err != nil {
		t.Fatal(err)
	}

	for _, addr := range addrs {
		has, err := client.Contains(net.ParseIP(addr))
		if err != nil {
			t.Error(err)
		}

		if has {
			t.Error("blocklist contains entry after remove")
		}
	}
}

func TestIP4Range(t *testing.T) {
	testRange(t, []string{"192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24"}, "192.0.2.0", "192.0.2.1", "198.51.100.0", "198.51.100.128", "203.0.113.0", "203.0.113.255")
}

func TestIP6Range(t *testing.T) {
	testRange(t, []string{"2001:db8::/112"}, "2001:db8::", "2001:db8::1", "2001:db8::f")
}

func TestIP6RouteRange(t *testing.T) {
	testRange(t, []string{"2001:db8::/58"}, "2001:db8::", "2001:db8::1", "2001:db8::f", "2001:db8::dead:beef")
}

func TestMixedRange(t *testing.T) {
	testRange(t, []string{"192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24", "2001:db8::/112", "::/64", "1::/58"}, "192.0.2.0", "192.0.2.1", "198.51.100.0", "198.51.100.128", "203.0.113.0", "203.0.113.255", "2001:db8::", "2001:db8::1", "2001:db8::f", "::", "::1", "::f", "::dead:beef", "1::", "1::1", "1::f", "1::dead:beef")
}

func testClear(t *testing.T, addrs ...string) {
	server, client, err := setup(true)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()
	defer client.Close()

	if err = server.Batch(); err != nil {
		t.Fatal(err)
	}

	for _, addr := range addrs {
		if err = server.Insert(net.ParseIP(addr)); err != nil {
			t.Error(err)
		}
	}

	if err = server.Clear(); err != nil {
		t.Fatal(err)
	}

	if err = server.Commit(); err != nil {
		t.Fatal(err)
	}

	for _, addr := range addrs {
		has, err := client.Contains(net.ParseIP(addr))
		if err != nil {
			t.Error(err)
		}

		if has {
			t.Error("blocklist contains entry after clear")
		}
	}
}

func TestClearIP4(t *testing.T) {
	testClear(t, "192.0.2.0", "192.0.2.1", "198.51.100.0", "198.51.100.1", "203.0.113.0", "203.0.113.1")
}

func TestClearIP6(t *testing.T) {
	testClear(t, "2001:db8::", "2001:db8::1", "2001:db8::f", "2001:db8::dead:beef")
}

func TestClearMixed(t *testing.T) {
	testClear(t, "192.0.2.0", "192.0.2.1", "198.51.100.0", "198.51.100.1", "203.0.113.0", "203.0.113.1", "2001:db8::", "2001:db8::1", "2001:db8::f", "2001:db8::dead:beef")
}

func TestClear2(t *testing.T) {
	server, _, err := setup(false)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	if err = server.Batch(); err != nil {
		t.Error(err)
	}

	extraIP := make(net.IP, net.IPv6len)

	for i := 0; i < 10000; i++ {
		rand.Read(extraIP)

		if err = server.Insert(extraIP[:net.IPv4len]); err != nil {
			t.Error(err)
		}

		if err = server.Insert(extraIP); err != nil {
			t.Error(err)
		}

		if err = server.InsertRange(extraIP, &net.IPNet{
			IP:   extraIP,
			Mask: net.CIDRMask(64, 128),
		}); err != nil {
			t.Error(err)
		}
	}

	// Clear with > pageSize bytes of data
	if err = server.Clear(); err != nil {
		t.Error(err)
	}

	ip4, ip6, ip6r, err := server.Count()
	if err != nil {
		t.Error(err)
	}

	if ip4 != 0 || ip6 != 0 || ip6r != 0 {
		t.Errorf("blocklist returned invalid count, expected (0, 0, 0), got (%d, %d, %d)", ip4, ip6, ip6r)
	}
}

func TestBatch(t *testing.T) {
	server, _, err := setup(false)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	if server.IsBatching() {
		t.Error("already batching")
	}

	if err = server.Batch(); err != nil {
		t.Error(err)
	}

	if err = server.Batch(); err != ErrAlreadyBatching {
		t.Error(err)
	}

	if !server.IsBatching() {
		t.Error("not batching")
	}

	if err = server.Insert(net.ParseIP("192.0.2.0")); err != nil {
		t.Error(err)
	}

	ip4, ip6, ip6r, err := server.Count()
	if err != nil {
		t.Error(err)
	}

	if ip4 != 0 || ip6 != 0 || ip6r != 0 {
		t.Errorf("blocklist returned invalid count, expected (0, 0, 0), got (%d, %d, %d)", ip4, ip6, ip6r)
	}

	if err = server.Commit(); err != nil {
		t.Error(err)
	}

	if err = server.Commit(); err != ErrNotBatching {
		t.Error(err)
	}

	if server.IsBatching() {
		t.Error("still batching")
	}

	ip4, ip6, ip6r, err = server.Count()
	if err != nil {
		t.Error(err)
	}

	if ip4 != 1 || ip6 != 0 || ip6r != 0 {
		t.Errorf("blocklist returned invalid count, expected (1, 0, 0), got (%d, %d, %d)", ip4, ip6, ip6r)
	}
}

func TestServerClose(t *testing.T) {
	server, _, err := setup(false)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	if err = server.Close(); err != nil {
		t.Error(err)
	}

	if err = server.Close(); err != ErrClosed {
		t.Error(err)
	}
}

func TestClientClose(t *testing.T) {
	server, client, err := setup(true)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()
	defer client.Close()

	if err = client.Close(); err != nil {
		t.Error(err)
	}

	if err = client.Close(); err != ErrClosed {
		t.Error(err)
	}
}

func TestUnlink(t *testing.T) {
	server, _, err := setup(false)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	if err = server.Unlink(); err != nil {
		t.Error(err)
	}

	if err = server.Close(); err != ErrClosed {
		t.Error(err)
	}

	if err = server.Unlink(); !os.IsNotExist(err) {
		t.Error(err)
	}

	server, _, err = setup(false)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	if err = Unlink(server.Name()); err != nil {
		t.Error(err)
	}

	if err = server.Unlink(); !os.IsNotExist(err) {
		t.Error(err)
	}

	if err = Unlink(server.Name()); !os.IsNotExist(err) {
		t.Error(err)
	}
}

func TestServerName(t *testing.T) {
	server, _, err := setup(false)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	if !strings.HasPrefix(server.Name(), "/go-test-") {
		t.Errorf("invalid name, expected of the form /go-test-%%d, got: %s", server.Name())
	}
}

func TestClientName(t *testing.T) {
	server, client, err := setup(true)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()
	defer client.Close()

	if !strings.HasPrefix(client.Name(), "/go-test-") {
		t.Errorf("invalid name, expected of the form /go-test-%%d, got: %s", client.Name())
	}

	if server.Name() != client.Name() {
		t.Errorf("client and server name do not match: %q != %q", server.Name(), client.Name())
	}
}

func TestServerCount(t *testing.T) {
	server, _, err := setup(false)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	cidr, cidrnet, err := net.ParseCIDR("2001:db8::/64")
	if err != nil {
		panic(err)
	}

	ip4, ip6, ip6r, err := server.Count()
	if err != nil {
		t.Error(err)
	}

	if ip4 != 0 || ip6 != 0 || ip6r != 0 {
		t.Errorf("blocklist returned invalid count, expected (0, 0, 0), got (%d, %d, %d)", ip4, ip6, ip6r)
	}

	if err = server.Insert(net.ParseIP("192.0.2.0")); err != nil {
		t.Error(err)
	}

	ip4, ip6, ip6r, err = server.Count()
	if err != nil {
		t.Error(err)
	}

	if ip4 != 1 || ip6 != 0 || ip6r != 0 {
		t.Errorf("blocklist returned invalid count, expected (1, 0, 0), got (%d, %d, %d)", ip4, ip6, ip6r)
	}

	if err = server.Insert(net.ParseIP("2001:db8::")); err != nil {
		t.Error(err)
	}

	ip4, ip6, ip6r, err = server.Count()
	if err != nil {
		t.Error(err)
	}

	if ip4 != 1 || ip6 != 1 || ip6r != 0 {
		t.Errorf("blocklist returned invalid count, expected (1, 1, 0), got (%d, %d, %d)", ip4, ip6, ip6r)
	}

	if err = server.InsertRange(cidr, cidrnet); err != nil {
		t.Error(err)
	}

	ip4, ip6, ip6r, err = server.Count()
	if err != nil {
		t.Error(err)
	}

	if ip4 != 1 || ip6 != 1 || ip6r != 1 {
		t.Errorf("blocklist returned invalid count, expected (1, 1, 1), got (%d, %d, %d)", ip4, ip6, ip6r)
	}

	if err = server.Remove(net.ParseIP("2001:db8::")); err != nil {
		t.Error(err)
	}

	ip4, ip6, ip6r, err = server.Count()
	if err != nil {
		t.Error(err)
	}

	if ip4 != 1 || ip6 != 0 || ip6r != 1 {
		t.Errorf("blocklist returned invalid count, expected (1, 0, 1), got (%d, %d, %d)", ip4, ip6, ip6r)
	}

	if err = server.RemoveRange(cidr, cidrnet); err != nil {
		t.Error(err)
	}

	ip4, ip6, ip6r, err = server.Count()
	if err != nil {
		t.Error(err)
	}

	if ip4 != 1 || ip6 != 0 || ip6r != 0 {
		t.Errorf("blocklist returned invalid count, expected (1, 0, 0), got (%d, %d, %d)", ip4, ip6, ip6r)
	}

	if err = server.Remove(net.ParseIP("192.0.2.0")); err != nil {
		t.Error(err)
	}

	ip4, ip6, ip6r, err = server.Count()
	if err != nil {
		t.Error(err)
	}

	if ip4 != 0 || ip6 != 0 || ip6r != 0 {
		t.Errorf("blocklist returned invalid count, expected (0, 0, 0), got (%d, %d, %d)", ip4, ip6, ip6r)
	}

	if err = server.Clear(); err != nil {
		t.Error(err)
	}

	ip4, ip6, ip6r, err = server.Count()
	if err != nil {
		t.Error(err)
	}

	if ip4 != 0 || ip6 != 0 || ip6r != 0 {
		t.Errorf("blocklist returned invalid count, expected (0, 0, 0), got (%d, %d, %d)", ip4, ip6, ip6r)
	}

	if err = server.Insert(net.ParseIP("2001:db8::")); err != nil {
		t.Error(err)
	}

	ip4, ip6, ip6r, err = server.Count()
	if err != nil {
		t.Error(err)
	}

	if ip4 != 0 || ip6 != 1 || ip6r != 0 {
		t.Errorf("blocklist returned invalid count, expected (0, 1, 0), got (%d, %d, %d)", ip4, ip6, ip6r)
	}

	if err = server.Insert(net.ParseIP("192.0.2.0")); err != nil {
		t.Error(err)
	}

	ip4, ip6, ip6r, err = server.Count()
	if err != nil {
		t.Error(err)
	}

	if ip4 != 1 || ip6 != 1 || ip6r != 0 {
		t.Errorf("blocklist returned invalid count, expected (1, 1, 0), got (%d, %d, %d)", ip4, ip6, ip6r)
	}

	if err = server.InsertRange(cidr, cidrnet); err != nil {
		t.Error(err)
	}

	ip4, ip6, ip6r, err = server.Count()
	if err != nil {
		t.Error(err)
	}

	if ip4 != 1 || ip6 != 1 || ip6r != 1 {
		t.Errorf("blocklist returned invalid count, expected (1, 1, 1), got (%d, %d, %d)", ip4, ip6, ip6r)
	}

	if err = server.Clear(); err != nil {
		t.Error(err)
	}

	ip4, ip6, ip6r, err = server.Count()
	if err != nil {
		t.Error(err)
	}

	if ip4 != 0 || ip6 != 0 || ip6r != 0 {
		t.Errorf("blocklist returned invalid count, expected (0, 0, 0), got (%d, %d, %d)", ip4, ip6, ip6r)
	}

	if server.Close(); err != nil {
		t.Error(err)
	}

	if _, _, _, err = server.Count(); err != ErrClosed {
		t.Error(err)
	}
}

func TestClientCount(t *testing.T) {
	server, client, err := setup(true)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()
	defer client.Close()

	ip4, ip6, ip6r, err := client.Count()
	if err != nil {
		t.Error(err)
	}

	if ip4 != 0 || ip6 != 0 || ip6r != 0 {
		t.Errorf("blocklist returned invalid count, expected (0, 0, 0), got (%d, %d, %d)", ip4, ip6, ip6r)
	}

	if err = server.Insert(net.ParseIP("192.0.2.0")); err != nil {
		t.Error(err)
	}

	if err = server.Insert(net.ParseIP("2001:db8::")); err != nil {
		t.Error(err)
	}

	cidr, cidrnet, err := net.ParseCIDR("2001:db8::/64")
	if err != nil {
		panic(err)
	}

	if err = server.InsertRange(cidr, cidrnet); err != nil {
		t.Error(err)
	}

	ip4, ip6, ip6r, err = client.Count()
	if err != nil {
		t.Error(err)
	}

	if ip4 != 1 || ip6 != 1 || ip6r != 1 {
		t.Errorf("blocklist returned invalid count, expected (1, 1, 1), got (%d, %d, %d)", ip4, ip6, ip6r)
	}

	if err = server.Clear(); err != nil {
		t.Error(err)
	}

	ip4, ip6, ip6r, err = client.Count()
	if err != nil {
		t.Error(err)
	}

	if ip4 != 0 || ip6 != 0 || ip6r != 0 {
		t.Errorf("blocklist returned invalid count, expected (0, 0, 0), got (%d, %d, %d)", ip4, ip6, ip6r)
	}

	if client.Close(); err != nil {
		t.Error(err)
	}

	if _, _, _, err = client.Count(); err != ErrClosed {
		t.Error(err)
	}
}

func TestClientCorrupted(t *testing.T) {
	server, client, err := setup(true)
	if err != nil {
		t.Fatal(err)
	}

	server.Close()
	server.Unlink()
	defer client.Close()

	client.data = client.data[:headerSize-1]

	if _, err = client.Contains(net.IPv4zero); err != ErrInvalidSharedMemory {
		t.Error(err)
	}

	if _, _, _, err = client.Count(); err != ErrInvalidSharedMemory {
		t.Error(err)
	}
}

func TestInsertRangeWithLastAlready(t *testing.T) {
	server, _, err := setup(false)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	if err = server.Batch(); err != nil {
		t.Fatal(err)
	}

	if err = server.Insert(net.ParseIP("192.0.2.253")); err != nil {
		t.Error(err)
	}

	if err = server.Insert(net.ParseIP("192.0.2.255")); err != nil {
		t.Error(err)
	}

	if err = server.Insert(net.ParseIP("192.0.3.0")); err != nil {
		t.Error(err)
	}

	ip, ipnet, err := net.ParseCIDR("192.0.2.0/24")
	if err != nil {
		panic(err)
	}

	if err = server.InsertRange(ip, ipnet); err != nil {
		t.Error(err)
	}

	if err = server.Commit(); err != nil {
		t.Fatal(err)
	}

	ip4, ip6, ip6r, err := server.Count()
	if err != nil {
		t.Error(err)
	}

	if ip4 != 257 || ip6 != 0 || ip6r != 0 {
		t.Errorf("blocklist returned invalid count, expected (257, 0, 0), got (%d, %d, %d)", ip4, ip6, ip6r)
	}
}

func TestRemoveRangeNonExistAtEnd(t *testing.T) {
	server, _, err := setup(false)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	if err = server.Batch(); err != nil {
		t.Fatal(err)
	}

	if err = server.Insert(net.ParseIP("192.0.1.0")); err != nil {
		t.Error(err)
	}

	ip, ipnet, err := net.ParseCIDR("192.0.2.0/30")
	if err != nil {
		panic(err)
	}

	if err = server.RemoveRange(ip, ipnet); err != nil {
		t.Error(err)
	}

	if err = server.Commit(); err != nil {
		t.Fatal(err)
	}

	ip4, ip6, ip6r, err := server.Count()
	if err != nil {
		t.Error(err)
	}

	if ip4 != 1 || ip6 != 0 || ip6r != 0 {
		t.Errorf("blocklist returned invalid count, expected (1, 0, 0), got (%d, %d, %d)", ip4, ip6, ip6r)
	}
}

func TestAddLargeRange(t *testing.T) {
	server, _, err := setup(false)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	ip, ipnet, err := net.ParseCIDR("192.0.2.0/12")
	if err != nil {
		panic(err)
	}

	if err = server.InsertRange(ip, ipnet); err != nil {
		t.Error(err)
	}

	ip4, ip6, ip6r, err := server.Count()
	if err != nil {
		t.Error(err)
	}

	if ip4 != 1<<20 || ip6 != 0 || ip6r != 0 {
		t.Errorf("blocklist returned invalid count, expected (1048576, 0, 0), got (%d, %d, %d)", ip4, ip6, ip6r)
	}
}

func TestOpenNonExist(t *testing.T) {
	client, err := Open("/go-test-bogus-name")
	if err == nil {
		client.Close()
	}

	if !os.IsNotExist(err) {
		t.Error("Open did not return non exist error for bogus name")
	}
}

func TestNewOpenEmptyName(t *testing.T) {
	client, err := Open("")
	if err == nil {
		client.Close()
	}

	if _, ok := err.(*os.PathError); !ok {
		t.Error("Open did not return *os.PathError for empty name")
	}

	server, err := New("", 0)
	if err == nil {
		server.Close()
		server.Unlink()
	}

	if _, ok := err.(*os.PathError); !ok {
		t.Error("New did not return *os.PathError for empty name")
	}
}

func TestUnlinkEmptyName(t *testing.T) {
	if _, ok := Unlink("").(*os.PathError); !ok {
		t.Errorf("Unlink did not return *os.PathError for empty name")
	}
}

func testPanic(fn func()) (didPanic bool) {
	defer func() {
		didPanic = recover() != nil
	}()

	fn()
	return
}

func TestClosedPanics(t *testing.T) {
	server, client, err := setup(true)
	if err != nil {
		t.Fatal(err)
	}

	client.Close()
	server.Close()
	server.Unlink()

	if !testPanic(func() {
		client.remap(true)
	}) {
		t.Error("(*Client).remap did not panic on closed")
	}

	if !testPanic(func() {
		client.checkSharedMemory()
	}) {
		t.Error("(*Client).checkSharedMemory did not panic on closed")
	}
}

func TestClosedErrors(t *testing.T) {
	server, client, err := setup(true)
	if err != nil {
		t.Fatal(err)
	}

	client.mu.RLock()
	done := make(chan struct{}, 1)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				t.Errorf("(*Client).remap panicked on closed with mutex lock, got %v", err)
			}

			done <- struct{}{}
		}()

		client.mu.RLock()
		if err := client.remap(false); err != ErrClosed {
			t.Errorf("(*Client).remap did not return ErrClosed on closed with mutex lock, got %v", err)
		} else {
			client.mu.RUnlock()
		}
	}()

	time.Sleep(50 * time.Millisecond)
	client.closed = true
	client.mu.RUnlock()
	<-done
	client.closed = false

	client.Close()
	server.Close()
	server.Unlink()

	if err = server.Commit(); err != ErrClosed {
		t.Errorf("(*Server).Commit did not return ErrClosed on closed, got %v", err)
	}

	if err = server.Insert(nil); err != ErrClosed {
		t.Errorf("(*Server).Insert did not return ErrClosed on closed, got %v", err)
	}

	if err = server.Remove(nil); err != ErrClosed {
		t.Errorf("(*Server).Remove did not return ErrClosed on closed, got %v", err)
	}

	if err = server.InsertRange(nil, nil); err != ErrClosed {
		t.Errorf("(*Server).Insert did not return ErrClosed on closed, got %v", err)
	}

	if err = server.RemoveRange(nil, nil); err != ErrClosed {
		t.Errorf("(*Server).Remove did not return ErrClosed on closed, got %v", err)
	}

	if err = server.Clear(); err != ErrClosed {
		t.Errorf("(*Server).Clear did not return ErrClosed on closed, got %v", err)
	}

	if err = server.Batch(); err != ErrClosed {
		t.Errorf("(*Server).Batch did not return ErrClosed on closed, got %v", err)
	}

	if err = server.Close(); err != ErrClosed {
		t.Errorf("(*Server).Close did not return ErrClosed on closed, got %v", err)
	}

	if _, _, _, err = server.Count(); err != ErrClosed {
		t.Errorf("(*Server).Count did not return ErrClosed on closed, got %v", err)
	}

	if _, err = client.Contains(nil); err != ErrClosed {
		t.Errorf("(*Client).Contains did not return ErrClosed on closed, got %v", err)
	}

	if err = client.Close(); err != ErrClosed {
		t.Errorf("(*Client).Close did not return ErrClosed on closed, got %v", err)
	}

	if _, _, _, err = client.Count(); err != ErrClosed {
		t.Errorf("(*Client).Count did not return ErrClosed on closed, got %v", err)
	}

	var b bytes.Buffer

	if err = server.Save(&b); err != ErrClosed {
		t.Errorf("(*Server).Save did not return ErrClosed on closed, got %v", err)
	}

	if err = server.Load(&b); err != ErrClosed {
		t.Errorf("(*Server).Load did not return ErrClosed on closed, got %v", err)
	}
}

func TestInvalidAddr(t *testing.T) {
	server, client, err := setup(true)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()
	defer client.Close()

	err = server.Insert(nil)
	if _, ok := err.(*net.AddrError); !ok {
		t.Error("(*Server).Insert did not return a net.AddrError with invalid address")
	}

	var invl [100]byte
	err = server.Insert(invl[:])
	if _, ok := err.(*net.AddrError); !ok {
		t.Error("(*Server).Insert did not return a net.AddrError with invalid address")
	}

	var invl2 [7]byte
	err = server.Insert(invl2[:])
	if _, ok := err.(*net.AddrError); !ok {
		t.Error("(*Server).Insert did not return a net.AddrError with invalid address")
	}

	err = server.Remove(invl[:])
	if _, ok := err.(*net.AddrError); !ok {
		t.Error("(*Server).Remove did not return a net.AddrError with invalid address")
	}

	err = server.InsertRange(nil, new(net.IPNet))
	if _, ok := err.(*net.AddrError); !ok {
		t.Error("(*Server).InsertRange did not return a net.AddrError with invalid address")
	}

	err = server.InsertRange(nil, &net.IPNet{
		IP:   net.IPv4zero,
		Mask: net.IPv4Mask(0xff, 0xff, 0xff, 0xff),
	})
	if _, ok := err.(*net.AddrError); !ok {
		t.Error("(*Server).InsertRange did not return a net.AddrError with invalid address")
	}

	err = server.InsertRange(invl[:], new(net.IPNet))
	if _, ok := err.(*net.AddrError); !ok {
		t.Error("(*Server).InsertRange did not return a net.AddrError with invalid address")
	}

	err = server.InsertRange(invl[:], &net.IPNet{
		IP:   net.IPv4zero,
		Mask: net.IPv4Mask(0xff, 0xff, 0xff, 0xff),
	})
	if _, ok := err.(*net.AddrError); !ok {
		t.Error("(*Server).InsertRange did not return a net.AddrError with invalid address")
	}

	err = server.InsertRange(invl2[:], new(net.IPNet))
	if _, ok := err.(*net.AddrError); !ok {
		t.Error("(*Server).InsertRange did not return a net.AddrError with invalid address")
	}

	err = server.InsertRange(invl2[:], &net.IPNet{
		IP:   net.IPv4zero,
		Mask: net.IPv4Mask(0xff, 0xff, 0xff, 0xff),
	})
	if _, ok := err.(*net.AddrError); !ok {
		t.Error("(*Server).InsertRange did not return a net.AddrError with invalid address")
	}

	err = server.RemoveRange(invl[:], new(net.IPNet))
	if _, ok := err.(*net.AddrError); !ok {
		t.Error("(*Server).RemoveRange did not return a net.AddrError with invalid address")
	}

	err = server.RemoveRange(invl[:], &net.IPNet{
		IP:   net.IPv4zero,
		Mask: net.IPv4Mask(0xff, 0xff, 0xff, 0xff),
	})
	if _, ok := err.(*net.AddrError); !ok {
		t.Error("(*Server).RemoveRange did not return a net.AddrError with invalid address")
	}

	_, err = client.Contains(nil)
	if _, ok := err.(*net.AddrError); !ok {
		t.Error("(*Client).Contains did not return a net.AddrError with invalid address")
	}

	_, err = client.Contains(invl[:])
	if _, ok := err.(*net.AddrError); !ok {
		t.Error("(*Client).Contains did not return a net.AddrError with invalid address")
	}

	_, err = client.Contains(invl2[:])
	if _, ok := err.(*net.AddrError); !ok {
		t.Error("(*Client).Contains did not return a net.AddrError with invalid address")
	}
}

func TestInvalidVersion(t *testing.T) {
	server, _, err := setup(false)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	header := castToHeader(&server.data[0])

	vx := version
	if version&0x80000000 == 0 {
		vx |= 0x80000000
	} else {
		vx &^= 0x80000000
	}

	for _, v := range [...]uint32{0xa5a5a5a5, 0, vx} {
		atomic.StoreUint32((*uint32)(&header.Version), v)

		client, err := Open(server.Name())
		if err != ErrInvalidSharedMemory {
			if err == nil {
				client.Close()
			}

			t.Error("Open did not return ErrInvalidSharedMemory for invalid version")
		}
	}
}

func TestMemoryTooShort(t *testing.T) {
	server, _, err := setup(false)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	if err = server.file.Truncate(headerSize - 1); err != nil {
		t.Fatal(err)
	}

	client, err := Open(server.Name())
	if err != ErrInvalidSharedMemory {
		if err == nil {
			client.Close()
		}

		t.Error("Open did not return ErrInvalidSharedMemory for too short memory")
	}
}

func TestInvalidHeader(t *testing.T) {
	server, _, err := setup(false)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	if err = server.file.Truncate(0xfffff); err != nil {
		t.Fatal(err)
	}

	header := castToHeader(&server.data[0])
	orig := *header

	const maxInt = int(^uint(0) >> 1)
	for i, fn := range [...]func(*shmHeader){
		func(h *shmHeader) { h.IP4.Base, h.IP4.Len = 0, 32 },
		func(h *shmHeader) { h.IP6.Base, h.IP6.Len = 0, 32 },
		func(h *shmHeader) { h.IP6Route.Base, h.IP6Route.Len = 0, 32 },
		func(h *shmHeader) { h.IP4.Base, h.IP4.Len = 0xfffff, 4<<10 },
		func(h *shmHeader) { h.IP6.Base, h.IP6.Len = 0xfffff, 16<<10 },
		func(h *shmHeader) { h.IP6Route.Base, h.IP6Route.Len = 0xfffff, 8<<10 },
		func(h *shmHeader) { h.IP4.Len = 7 },
		func(h *shmHeader) { h.IP6.Len = 31 },
		func(h *shmHeader) { h.IP6Route.Len = 15 },
		func(h *shmHeader) {
			h.setBlocks(int(h.IP4.Base), maxInt, int(h.IP6.Base), maxInt, int(h.IP6Route.Base), maxInt)
		},
		func(h *shmHeader) {
			h.setBlocks(int(h.IP4.Base), maxInt, int(h.IP6.Base), int(h.IP6.Len), int(h.IP6Route.Base), int(h.IP6Route.Len))
		},
		func(h *shmHeader) {
			h.setBlocks(int(h.IP4.Base), int(h.IP4.Len), int(h.IP6.Base), maxInt, int(h.IP6Route.Base), int(h.IP6Route.Len))
		},
		func(h *shmHeader) {
			h.setBlocks(int(h.IP4.Base), int(h.IP4.Len), int(h.IP6.Base), int(h.IP6.Len), int(h.IP6Route.Base), maxInt)
		},
		func(h *shmHeader) {
			h.setBlocks(int(h.IP4.Base), 0xfffff, int(h.IP6.Base), int(h.IP6.Len), int(h.IP6Route.Base), int(h.IP6Route.Len))
		},
		func(h *shmHeader) {
			h.setBlocks(int(h.IP4.Base), int(h.IP4.Len), int(h.IP6.Base), 0xfffff, int(h.IP6Route.Base), int(h.IP6Route.Len))
		},
		func(h *shmHeader) {
			h.setBlocks(int(h.IP4.Base), int(h.IP4.Len), int(h.IP6.Base), int(h.IP6.Len), int(h.IP6Route.Base), 0xfffff)
		},
	} {
		fn(header)

		client, err := Open(server.Name())
		if err != ErrInvalidSharedMemory {
			if err == nil {
				client.Close()
			}

			t.Errorf("Open did not return ErrInvalidSharedMemory for invalid header (%d)", i)
		}

		lock := header.Lock
		*header = orig
		header.Lock = lock
	}
}

func testChangeBeforeLock(t *testing.T, corrupter func(*shmHeader)) {
	server, _, err := setup(false)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	header := castToHeader(&server.data[0])
	lock := (*rwLock)(&header.Lock)
	lock.Lock()

	done := make(chan struct{}, 1)
	go func() {
		defer func() {
			done <- struct{}{}
		}()

		client, err := Open(server.Name())
		if corrupter != nil {
			if err != ErrInvalidSharedMemory {
				if err == nil {
					client.Close()
				}

				t.Error("Open did not return ErrInvalidSharedMemory for invalid header")
			}
		} else {
			if err != nil {
				t.Error(err)
			} else {
				client.Close()
			}
		}
	}()

	time.Sleep(50 * time.Millisecond)

	if err = server.file.Truncate(int64(len(server.data)) + 1); err != nil {
		t.Fatal(err)
	}

	if corrupter != nil {
		corrupter(header)
	}

	lock.Unlock()
	<-done
}

func TestChangeBeforeLock(t *testing.T) {
	testChangeBeforeLock(t, nil)
}

func TestCorruptChangeBeforeLock(t *testing.T) {
	testChangeBeforeLock(t, func(h *shmHeader) {
		h.IP4.Base, h.IP4.Len = 0, 32
	})
}

func TestCorruptContains(t *testing.T) {
	server, client, err := setup(true)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()
	defer client.Close()

	header := castToHeader(&server.data[0])

	lock := (*rwLock)(&header.Lock)
	lock.Lock()

	header.IP6.Base, header.IP6.Len = 0, 32
	header.Revision++

	lock.Unlock()

	if _, err := client.Contains(net.IPv4zero); err != ErrInvalidSharedMemory {
		t.Error("Contains did not return ErrInvalidSharedMemory for corrupt header")
	}
}

func TestClientRemapAlreadyDone(t *testing.T) {
	server, client, err := setup(true)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()
	defer client.Close()

	client.mu.RLock()
	client.mu.RLock()

	go func() {
		defer client.mu.RUnlock()

		if err := client.remap(true); err != nil {
			t.Error(err)
		}
	}()

	defer client.mu.RUnlock()

	if err := client.remap(false); err != nil {
		t.Error(err)
	}
}

func TestClientRemapLockFailure(t *testing.T) {
	server, client, err := setup(true)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()
	defer client.Close()

	if err = server.file.Truncate(headerSize - 1); err != nil {
		t.Fatal(err)
	}

	if _, ok := client.remap(true).(LockReleaseFailedError); !ok {
		t.Error("remap managed to release invalid lock")
	}
}

func TestInsertRangeMassive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	var mask string
	var expect int

	switch os.Getenv("INSERTMASSIVERANGETEST") {
	case "1":
		mask = "/2"
		expect = 1073741824
	case "2":
		mask = "/1"
		expect = 2147483648

		if ^uint(0) == uint(^uint32(0)) {
			t.Skip("INSERTMASSIVERANGETEST=2 cannot be run on 32-bit systems")
		}
	case "3":
		mask = "/0"

		expect = 4294967296 - 1
		expect++

		if ^uint(0) == uint(^uint32(0)) {
			t.Skip("INSERTMASSIVERANGETEST=3 cannot be run on 32-bit systems")
		}
	default:
		t.Skip("INSERTMASSIVERANGETEST is not set to 1, 2 or 3")
	}

	server, _, err := setup(false)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	if err = server.Batch(); err != nil {
		t.Error(err)
	}

	ip, ipnet, err := net.ParseCIDR("192.0.2.0" + mask)
	if err != nil {
		panic(err)
	}

	if err = server.InsertRange(ip, ipnet); err != nil {
		t.Error(err)
	}

	if c := len(server.ip4s.Data) / net.IPv4len; c != expect {
		t.Errorf("InsertRange(192.0.2.0%s) failed, expected count of %d ip4 address, got %d", mask, expect, c)
	}
}

func TestLoadSave(t *testing.T) {
	server1, _, err := setup(false)
	if err != nil {
		t.Fatal(err)
	}

	defer server1.Unlink()
	defer server1.Close()

	server2, _, err := setup(false)
	if err != nil {
		t.Fatal(err)
	}

	defer server2.Unlink()
	defer server2.Close()

	if err = server1.Batch(); err != nil {
		t.Error(err)
	}

	if err = server2.Batch(); err != nil {
		t.Error(err)
	}

	extraIP := make(net.IP, net.IPv6len)

	for i := 0; i < 10000; i++ {
		rand.Read(extraIP)

		if err = server1.Insert(extraIP[:net.IPv4len]); err != nil {
			t.Error(err)
		}

		if err = server1.Insert(extraIP); err != nil {
			t.Error(err)
		}

		if err = server1.InsertRange(extraIP, &net.IPNet{
			IP:   extraIP,
			Mask: net.CIDRMask(64, 128),
		}); err != nil {
			t.Error(err)
		}
	}

	var b bytes.Buffer

	if err = server1.Save(&b); err != nil {
		t.Fatal(err)
	}

	if err = server2.Load(&b); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(server1.ip4s.Data, server2.ip4s.Data) {
		t.Errorf("ip4 data differs after Load, Save")
	}

	if !bytes.Equal(server1.ip6s.Data, server2.ip6s.Data) {
		t.Errorf("ip6 data differs after Load, Save")
	}

	if !bytes.Equal(server1.ip6rs.Data, server2.ip6rs.Data) {
		t.Errorf("ip6 route data differs after Load, Save")
	}
}

func TestLoadNotBatching(t *testing.T) {
	server, _, err := setup(false)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	if err = server.Batch(); err != nil {
		t.Error(err)
	}

	if err = server.Insert(net.ParseIP("192.0.2.0")); err != nil {
		t.Error(err)
	}

	var b bytes.Buffer

	if err = server.Save(&b); err != nil {
		t.Error(err)
	}

	if err = server.Clear(); err != nil {
		t.Error(err)
	}

	if err = server.Commit(); err != nil {
		t.Error(err)
	}

	if err = server.Load(&b); err != nil {
		t.Error(err)
	}
}

func BenchmarkNew(b *testing.B) {
	name := fmt.Sprintf("/go-test-%d", nameRand.Int())

	for i := 0; i < b.N; i++ {
		server, err := New(name, 0600)
		if err != nil {
			b.Error(err)
			continue
		}

		b.StopTimer()

		server.Close()
		server.Unlink()

		b.StartTimer()
	}
}

func BenchmarkOpen(b *testing.B) {
	server, _, err := setup(false)
	if err != nil {
		b.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	name := server.Name()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		client, err := Open(name)
		if err != nil {
			b.Error(err)
			continue
		}

		b.StopTimer()

		client.Close()

		b.StartTimer()
	}
}

func benchmarkInsert(b *testing.B, addr string, extra int) {
	server, _, err := setup(false)
	if err != nil {
		b.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	if err = server.Batch(); err != nil {
		b.Error(err)
	}

	ip := net.ParseIP(addr)
	if ip == nil {
		panic("failed to parse " + addr)
	}

	extraIP := make(net.IP, len(ip))

	for i := 0; i < extra; i++ {
		rand.Read(extraIP)

		if err = server.Insert(extraIP); err != nil {
			b.Error(err)
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err = server.Insert(ip); err != nil {
			b.Error(err)
		}

		b.StopTimer()

		if err = server.Remove(ip); err != nil {
			b.Error(err)
		}

		b.StartTimer()
	}
}

func BenchmarkInsertIP4NoSearch(b *testing.B) {
	benchmarkInsert(b, "192.0.2.0", 0)
}

func BenchmarkInsertIP6NoSearch(b *testing.B) {
	benchmarkInsert(b, "2001:db8::", 0)
}

func BenchmarkInsertIP4(b *testing.B) {
	benchmarkInsert(b, "192.0.2.0", 100000)
}

func BenchmarkInsertIP6(b *testing.B) {
	benchmarkInsert(b, "2001:db8::", 100000)
}

func benchmarkRemove(b *testing.B, addr string, extra int) {
	server, _, err := setup(false)
	if err != nil {
		b.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	if err = server.Batch(); err != nil {
		b.Error(err)
	}

	ip := net.ParseIP(addr)
	if ip == nil {
		panic("failed to parse " + addr)
	}

	extraIP := make(net.IP, len(ip))

	for i := 0; i < extra; i++ {
		rand.Read(extraIP)

		if err = server.Insert(extraIP); err != nil {
			b.Error(err)
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()

		if err = server.Insert(ip); err != nil {
			b.Error(err)
		}

		b.StartTimer()

		if err = server.Remove(ip); err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkRemoveIP4NoSearch(b *testing.B) {
	benchmarkRemove(b, "192.0.2.0", 0)
}

func BenchmarkRemoveIP6NoSearch(b *testing.B) {
	benchmarkRemove(b, "2001:db8::", 0)
}

func BenchmarkRemoveIP4(b *testing.B) {
	benchmarkRemove(b, "192.0.2.0", 100000)
}

func BenchmarkRemoveIP6(b *testing.B) {
	benchmarkRemove(b, "2001:db8::", 100000)
}

func benchmarkContains(b *testing.B, addr string, extra int) {
	server, client, err := setup(true)
	if err != nil {
		b.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()
	defer client.Close()

	ip := net.ParseIP(addr)
	if ip == nil {
		panic("failed to parse " + addr)
	}

	if err = server.Batch(); err != nil {
		b.Error(err)
	}

	if err = server.Insert(ip); err != nil {
		b.Error(err)
	}

	extraIP := make(net.IP, len(ip))

	for i := 0; i < extra; i++ {
		rand.Read(extraIP)

		if err = server.Insert(extraIP); err != nil {
			b.Error(err)
		}
	}

	if err = server.Commit(); err != nil {
		b.Error(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		has, err := client.Contains(ip)
		if err != nil {
			b.Error(err)
		}

		if !has {
			b.Error("blocklist does not contain IP")
		}
	}
}

func BenchmarkContainsIP4NoSearch(b *testing.B) {
	benchmarkContains(b, "192.0.2.0", 0)
}

func BenchmarkContainsIP6NoSearch(b *testing.B) {
	benchmarkContains(b, "2001:db8::", 0)
}

func BenchmarkContainsIP4(b *testing.B) {
	benchmarkContains(b, "192.0.2.0", 100000)
}

func BenchmarkContainsIP6(b *testing.B) {
	benchmarkContains(b, "2001:db8::", 100000)
}

func benchmarkCommit(b *testing.B, extra int) {
	server, _, err := setup(false)
	if err != nil {
		b.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	extraIP := make(net.IP, net.IPv6len)

	if err = server.Batch(); err != nil {
		b.Fatal(err)
	}

	for i := 0; i < extra; i++ {
		rand.Read(extraIP)

		if err = server.Insert(extraIP[:net.IPv4len]); err != nil {
			b.Error(err)
		}

		if err = server.Insert(extraIP); err != nil {
			b.Error(err)
		}

		if err = server.InsertRange(extraIP, &net.IPNet{
			IP:   extraIP,
			Mask: net.CIDRMask(64, 128),
		}); err != nil {
			b.Error(err)
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err = server.Commit(); err != nil {
			b.Error(err)
		}

		b.StopTimer()

		if err = server.Batch(); err != nil {
			b.Error(err)
		}

		b.StartTimer()
	}
}

func BenchmarkCommitEmpty(b *testing.B) {
	benchmarkCommit(b, 0)
}

func BenchmarkCommit(b *testing.B) {
	benchmarkCommit(b, 100000/3)
}

func BenchmarkClientRemap(b *testing.B) {
	server, client, err := setup(true)
	if err != nil {
		b.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()
	defer client.Close()

	header := castToHeader(&client.data[0])
	lock := (*rwLock)(&header.Lock)
	lock.RLock()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err = client.remap(true); err != nil {
			b.Fatal(err)
		}
	}

	b.StopTimer()

	header = castToHeader(&client.data[0])
	lock = (*rwLock)(&header.Lock)
	lock.RUnlock()
}

func benchmarkInsertRemoveRange(b *testing.B, insert bool, iprange string, extra int) {
	server, _, err := setup(false)
	if err != nil {
		b.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	if err = server.Batch(); err != nil {
		b.Error(err)
	}

	ip, ipnet, err := net.ParseCIDR(iprange)
	if err != nil {
		panic(err)
	}

	extraIP := make(net.IP, len(ip))

	for i := 0; i < extra; i++ {
		rand.Read(extraIP)

		if err = server.Insert(extraIP[:net.IPv4len]); err != nil {
			b.Error(err)
		}

		if err = server.Insert(extraIP); err != nil {
			b.Error(err)
		}

		if err = server.InsertRange(extraIP, &net.IPNet{
			IP:   extraIP,
			Mask: net.CIDRMask(64, 128),
		}); err != nil {
			b.Error(err)
		}
	}

	b.ResetTimer()

	if insert {
		for i := 0; i < b.N; i++ {
			if err = server.InsertRange(ip, ipnet); err != nil {
				b.Error(err)
			}

			b.StopTimer()

			if err = server.RemoveRange(ip, ipnet); err != nil {
				b.Error(err)
			}

			b.StartTimer()
		}
	} else {
		for i := 0; i < b.N; i++ {
			b.StopTimer()

			if err = server.InsertRange(ip, ipnet); err != nil {
				b.Error(err)
			}

			b.StartTimer()

			if err = server.RemoveRange(ip, ipnet); err != nil {
				b.Error(err)
			}
		}
	}
}

func BenchmarkInsertRangeIP4NoSearch20(b *testing.B) {
	benchmarkInsertRemoveRange(b, true, "192.0.2.0/20", 0)
}

func BenchmarkInsertRangeIP4NoSearch8(b *testing.B) {
	benchmarkInsertRemoveRange(b, true, "192.0.2.0/8", 0)
}

func BenchmarkInsertRangeIP6NoSearch116(b *testing.B) {
	benchmarkInsertRemoveRange(b, true, "2001:db8::/116", 0)
}

func BenchmarkInsertRangeIP6RouteNoSearch52(b *testing.B) {
	benchmarkInsertRemoveRange(b, true, "2001:db8::/52", 0)
}

func BenchmarkInsertRangeIP420(b *testing.B) {
	benchmarkInsertRemoveRange(b, true, "192.0.2.0/20", 100000)
}

func BenchmarkInsertRangeIP48(b *testing.B) {
	benchmarkInsertRemoveRange(b, true, "192.0.2.0/8", 100000)
}

func BenchmarkInsertRangeIP6116(b *testing.B) {
	benchmarkInsertRemoveRange(b, true, "2001:db8::/116", 100000)
}

func BenchmarkInsertRangeIP6Route52(b *testing.B) {
	benchmarkInsertRemoveRange(b, true, "2001:db8::/52", 100000)
}

func BenchmarkRemoveRangeIP4NoSearch20(b *testing.B) {
	benchmarkInsertRemoveRange(b, false, "192.0.2.0/20", 0)
}

func BenchmarkRemoveRangeIP4NoSearch8(b *testing.B) {
	benchmarkInsertRemoveRange(b, false, "192.0.2.0/8", 0)
}

func BenchmarkRemoveRangeIP6NoSearch116(b *testing.B) {
	benchmarkInsertRemoveRange(b, false, "2001:db8::/116", 0)
}

func BenchmarkRemoveRangeIP6RouteNoSearch52(b *testing.B) {
	benchmarkInsertRemoveRange(b, false, "2001:db8::/52", 0)
}

func BenchmarkRemoveRangeIP420(b *testing.B) {
	benchmarkInsertRemoveRange(b, false, "192.0.2.0/20", 100000)
}

func BenchmarkRemoveRangeIP48(b *testing.B) {
	benchmarkInsertRemoveRange(b, false, "192.0.2.0/8", 100000)
}

func BenchmarkRemoveRangeIP6116(b *testing.B) {
	benchmarkInsertRemoveRange(b, false, "2001:db8::/116", 100000)
}

func BenchmarkRemoveRangeIP6Route52(b *testing.B) {
	benchmarkInsertRemoveRange(b, false, "2001:db8::/52", 100000)
}

func benchmarkSave(b *testing.B, extra int) {
	server, _, err := setup(false)
	if err != nil {
		b.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	if err = server.Batch(); err != nil {
		b.Error(err)
	}

	extraIP := make(net.IP, net.IPv6len)

	for i := 0; i < extra; i++ {
		rand.Read(extraIP)

		if err = server.Insert(extraIP[:net.IPv4len]); err != nil {
			b.Error(err)
		}

		if err = server.Insert(extraIP); err != nil {
			b.Error(err)
		}

		if err = server.InsertRange(extraIP, &net.IPNet{
			IP:   extraIP,
			Mask: net.CIDRMask(64, 128),
		}); err != nil {
			b.Error(err)
		}
	}

	b.ResetTimer()

	var buf bytes.Buffer

	for i := 0; i < b.N; i++ {
		buf.Reset()

		if err = server.Save(&buf); err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkSaveEmpty(b *testing.B) {
	benchmarkSave(b, 0)
}

func BenchmarkSave(b *testing.B) {
	benchmarkSave(b, 100000/3)
}

func benchmarkLoad(b *testing.B, extra int) {
	server, _, err := setup(false)
	if err != nil {
		b.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	if err = server.Batch(); err != nil {
		b.Error(err)
	}

	extraIP := make(net.IP, net.IPv6len)

	for i := 0; i < extra; i++ {
		rand.Read(extraIP)

		if err = server.Insert(extraIP[:net.IPv4len]); err != nil {
			b.Error(err)
		}

		if err = server.Insert(extraIP); err != nil {
			b.Error(err)
		}

		if err = server.InsertRange(extraIP, &net.IPNet{
			IP:   extraIP,
			Mask: net.CIDRMask(64, 128),
		}); err != nil {
			b.Error(err)
		}
	}

	var buf bytes.Buffer

	if err = server.Save(&buf); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()

		if err = server.Clear(); err != nil {
			b.Fatal(err)
		}

		b.StartTimer()

		buf2 := buf

		if err = server.Load(&buf2); err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkLoadEmpty(b *testing.B) {
	benchmarkLoad(b, 0)
}

func BenchmarkLoad(b *testing.B) {
	benchmarkLoad(b, 100000/3)
}
