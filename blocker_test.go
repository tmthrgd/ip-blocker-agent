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
	"syscall"
	"testing"
	"time"
	"unsafe"
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

func insertRemoveRangeSlowHook(insert bool, ip net.IP, ipnet *net.IPNet, ips *binarySearcher) {
	size := ips.Size()

	if insert {
		for ; ipnet.Contains(ip); incrBytes(ip[:size]) {
			ips.Insert(ip[:size])
		}
	} else {
		for ; ipnet.Contains(ip); incrBytes(ip[:size]) {
			ips.Remove(ip[:size])
		}
	}
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

		if err = server.Insert(net.ParseIP(addr)); err != nil {
			t.Error(err)
			continue
		}

		has, err = client.Contains(net.ParseIP(addr))
		if err != nil {
			t.Error(err)
			continue
		}

		if !has {
			t.Error("blocklist does not contain entry after insert")
			continue
		}

		if err = server.Remove(net.ParseIP(addr)); err != nil {
			t.Error(err)
			continue
		}

		has, err = client.Contains(net.ParseIP(addr))
		if err != nil {
			t.Error(err)
		}

		if has {
			t.Error("blocklist contains entry after remove")
		}
	}
}

func TestIP4(t *testing.T) {
	t.Parallel()

	testAddress(t, "192.0.2.0", "192.0.2.1", "198.51.100.0", "198.51.100.1", "203.0.113.0", "203.0.113.1")
}

func TestIP6(t *testing.T) {
	t.Parallel()

	testAddress(t, "2001:db8::", "2001:db8::1", "2001:db8::f", "2001:db8::dead:beef")
}

func TestMixed(t *testing.T) {
	t.Parallel()

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

	for _, iprange := range ipranges {
		ip, ipnet, err := net.ParseCIDR(iprange)
		if err != nil {
			panic(err)
		}

		if err = server.InsertRange(ip, ipnet); err != nil {
			t.Fatal(err)
		}
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

	for _, iprange := range ipranges {
		ip, ipnet, err := net.ParseCIDR(iprange)
		if err != nil {
			panic(err)
		}

		if err = server.RemoveRange(ip, ipnet); err != nil {
			t.Fatal(err)
		}
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
	t.Parallel()

	testRange(t, []string{"192.0.2.0/24"}, "192.0.2.0", "192.0.2.1")
	testRange(t, []string{"198.51.100.0/24"}, "198.51.100.0", "198.51.100.128")
	testRange(t, []string{"203.0.113.0/24"}, "203.0.113.0", "203.0.113.255")
	testRange(t, []string{"192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24"}, "192.0.2.0", "192.0.2.1", "198.51.100.0", "198.51.100.128", "203.0.113.0", "203.0.113.255")
}

func TestIP6Range(t *testing.T) {
	t.Parallel()

	testRange(t, []string{"2001:db8::/112"}, "2001:db8::", "2001:db8::1", "2001:db8::f")
}

func TestIP6RouteRange(t *testing.T) {
	t.Parallel()

	testRange(t, []string{"2001:db8::/64"}, "2001:db8::", "2001:db8::1", "2001:db8::f", "2001:db8::dead:beef")
	testRange(t, []string{"2001:db8::/58"}, "2001:db8::", "2001:db8::1", "2001:db8::f", "2001:db8::dead:beef")
}

func TestMixedRange(t *testing.T) {
	t.Parallel()

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

	for _, addr := range addrs {
		has, err := client.Contains(net.ParseIP(addr))
		if err != nil {
			t.Error(err)
		}

		if has {
			t.Error("blocklist contains entry before any added")
		}

		if err = server.Insert(net.ParseIP(addr)); err != nil {
			t.Error(err)
			continue
		}

		has, err = client.Contains(net.ParseIP(addr))
		if err != nil {
			t.Error(err)
		}

		if !has {
			t.Error("blocklist does not contain entry after insert")
		}
	}

	if err = server.Clear(); err != nil {
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
	t.Parallel()

	testClear(t, "192.0.2.0", "192.0.2.1", "198.51.100.0", "198.51.100.1", "203.0.113.0", "203.0.113.1")
}

func TestClearIP6(t *testing.T) {
	t.Parallel()

	testClear(t, "2001:db8::", "2001:db8::1", "2001:db8::f", "2001:db8::dead:beef")
}

func TestClearMixed(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

func testBinarySearcherInsertRange(t *testing.T, extra int, ipranges ...string) {
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

	server1.doInsertRemoveRangeHook = insertRemoveRangeSlowHook
	server2.doInsertRemoveRangeHook = insertRemoveRangeSlowHook

	extraIP := make(net.IP, net.IPv6len)

	for i := 0; i < extra; i++ {
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

		if err = server2.Insert(extraIP[:net.IPv4len]); err != nil {
			t.Error(err)
		}

		if err = server2.Insert(extraIP); err != nil {
			t.Error(err)
		}

		if err = server2.InsertRange(extraIP, &net.IPNet{
			IP:   extraIP,
			Mask: net.CIDRMask(64, 128),
		}); err != nil {
			t.Error(err)
		}
	}

	server2.doInsertRemoveRangeHook = nil

	for _, iprange := range ipranges {
		ip, ipnet, err := net.ParseCIDR(iprange)
		if err != nil {
			panic(err)
		}

		if rand.Intn(2) == 0 {
			if err = server1.Insert(ip); err != nil {
				t.Error(err)
			}

			if err = server2.Insert(ip); err != nil {
				t.Error(err)
			}

			for j := 0; j < rand.Intn(4); j++ {
				incrBytes(ip)
			}

			for j := 0; j < rand.Intn(6); j++ {
				incrBytes(ip)

				if err = server1.Insert(ip); err != nil {
					t.Error(err)
				}

				if err = server2.Insert(ip); err != nil {
					t.Error(err)
				}
			}
		}

		if err = server1.InsertRange(ip, ipnet); err != nil {
			t.Error(err)
		}

		if err = server2.InsertRange(ip, ipnet); err != nil {
			t.Error(err)
		}
	}

	if !bytes.Equal(server1.ip4s.Data, server2.ip4s.Data) {
		t.Errorf("doInsertRemoveRangeHook = nil and = insertRemoveRangeSlowHook produced different ip4 data, with extra = %d", extra)
	}

	if !bytes.Equal(server1.ip6s.Data, server2.ip6s.Data) {
		t.Errorf("doInsertRemoveRangeHook = nil and = insertRemoveRangeSlowHook produced different ip6 data, with extra = %d", extra)
	}

	if !bytes.Equal(server1.ip6rs.Data, server2.ip6rs.Data) {
		t.Errorf("doInsertRemoveRangeHook = nil and = insertRemoveRangeSlowHook produced different ip6 route data, with extra = %d", extra)
	}
}

func TestBinarySearcherInsertRangeIP4NoSearch(t *testing.T) {
	t.Parallel()

	testBinarySearcherInsertRange(t, 0, "192.0.2.0/24")
	testBinarySearcherInsertRange(t, 0, "198.51.100.0/24")
	testBinarySearcherInsertRange(t, 0, "203.0.113.0/24")
	testBinarySearcherInsertRange(t, 0, "192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24")
}

func TestBinarySearcherInsertRangeIP6NoSearch(t *testing.T) {
	t.Parallel()

	testBinarySearcherInsertRange(t, 0, "2001:db8::/112")
}

func TestBinarySearcherInsertRangeIP6RouteNoSearch(t *testing.T) {
	t.Parallel()

	testBinarySearcherInsertRange(t, 0, "2001:db8::/64")
	testBinarySearcherInsertRange(t, 0, "2001:db8::/58")
}

func TestBinarySearcherInsertRangeMixedNoSearch(t *testing.T) {
	t.Parallel()

	testBinarySearcherInsertRange(t, 0, "192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24", "2001:db8::/112", "::/64", "1::/58")
}

func TestBinarySearcherInsertRangeIP4(t *testing.T) {
	t.Parallel()

	testBinarySearcherInsertRange(t, 10000, "192.0.2.0/24")
	testBinarySearcherInsertRange(t, 10000, "198.51.100.0/24")
	testBinarySearcherInsertRange(t, 10000, "203.0.113.0/24")
	testBinarySearcherInsertRange(t, 10000, "192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24")
}

func TestBinarySearcherInsertRangeIP6(t *testing.T) {
	t.Parallel()

	testBinarySearcherInsertRange(t, 10000, "2001:db8::/112")
}

func TestBinarySearcherInsertRangeIP6Route(t *testing.T) {
	t.Parallel()

	testBinarySearcherInsertRange(t, 10000, "2001:db8::/64")
	testBinarySearcherInsertRange(t, 10000, "2001:db8::/58")
}

func TestBinarySearcherInsertRangeMixed(t *testing.T) {
	t.Parallel()

	testBinarySearcherInsertRange(t, 10000, "192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24", "2001:db8::/112", "::/64", "1::/58")
}

func testBinarySearcherRemoveRange(t *testing.T, extra int, ipranges ...string) {
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

	for i := 0; i < extra; i++ {
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

		if err = server2.Insert(extraIP[:net.IPv4len]); err != nil {
			t.Error(err)
		}

		if err = server2.Insert(extraIP); err != nil {
			t.Error(err)
		}

		if err = server2.InsertRange(extraIP, &net.IPNet{
			IP:   extraIP,
			Mask: net.CIDRMask(64, 128),
		}); err != nil {
			t.Error(err)
		}
	}

	for _, iprange := range ipranges {
		ip, ipnet, err := net.ParseCIDR(iprange)
		if err != nil {
			panic(err)
		}

		if err = server1.InsertRange(ip, ipnet); err != nil {
			t.Error(err)
		}

		if err = server2.InsertRange(ip, ipnet); err != nil {
			t.Error(err)
		}

		if rand.Intn(2) == 0 {
			for j := 0; j < 1+rand.Intn(5); j++ {
				incrBytes(ip)
			}

			if err = server1.Remove(ip); err != nil {
				t.Error(err)
			}

			if err = server2.Remove(ip); err != nil {
				t.Error(err)
			}
		}
	}

	server1.doInsertRemoveRangeHook = insertRemoveRangeSlowHook

	for _, iprange := range ipranges {
		ip, ipnet, err := net.ParseCIDR(iprange)
		if err != nil {
			panic(err)
		}

		if err = server1.RemoveRange(ip, ipnet); err != nil {
			t.Error(err)
		}

		if err = server2.RemoveRange(ip, ipnet); err != nil {
			t.Error(err)
		}
	}

	if !bytes.Equal(server1.ip4s.Data, server2.ip4s.Data) {
		t.Errorf("doInsertRemoveRangeHook = nil and = insertRemoveRangeSlowHook produced different ip4 data, with extra = %d", extra)
	}

	if !bytes.Equal(server1.ip6s.Data, server2.ip6s.Data) {
		t.Errorf("doInsertRemoveRangeHook = nil and = insertRemoveRangeSlowHook produced different ip6 data, with extra = %d", extra)
	}

	if !bytes.Equal(server1.ip6rs.Data, server2.ip6rs.Data) {
		t.Errorf("doInsertRemoveRangeHook = nil and = insertRemoveRangeSlowHook produced different ip6 route data, with extra = %d", extra)
	}
}

func TestBinarySearcherRemoveRangeIP4NoSearch(t *testing.T) {
	t.Parallel()

	testBinarySearcherRemoveRange(t, 0, "192.0.2.0/24")
	testBinarySearcherRemoveRange(t, 0, "198.51.100.0/24")
	testBinarySearcherRemoveRange(t, 0, "203.0.113.0/24")
	testBinarySearcherRemoveRange(t, 0, "192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24")
}

func TestBinarySearcherRemoveRangeIP6NoSearch(t *testing.T) {
	t.Parallel()

	testBinarySearcherRemoveRange(t, 0, "2001:db8::/112")
}

func TestBinarySearcherRemoveRangeIP6RouteNoSearch(t *testing.T) {
	t.Parallel()

	testBinarySearcherRemoveRange(t, 0, "2001:db8::/64")
	testBinarySearcherRemoveRange(t, 0, "2001:db8::/58")
}

func TestBinarySearcherRemoveRangeMixedNoSearch(t *testing.T) {
	t.Parallel()

	testBinarySearcherRemoveRange(t, 0, "192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24", "2001:db8::/112", "::/64", "1::/58")
}

func TestBinarySearcherRemoveRangeIP4(t *testing.T) {
	t.Parallel()

	testBinarySearcherRemoveRange(t, 10000, "192.0.2.0/24")
	testBinarySearcherRemoveRange(t, 10000, "198.51.100.0/24")
	testBinarySearcherRemoveRange(t, 10000, "203.0.113.0/24")
	testBinarySearcherRemoveRange(t, 10000, "192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24")
}

func TestBinarySearcherRemoveRangeIP6(t *testing.T) {
	t.Parallel()

	testBinarySearcherRemoveRange(t, 10000, "2001:db8::/112")
}

func TestBinarySearcherRemoveRangeIP6Route(t *testing.T) {
	t.Parallel()

	testBinarySearcherRemoveRange(t, 10000, "2001:db8::/64")
	testBinarySearcherRemoveRange(t, 10000, "2001:db8::/58")
}

func TestBinarySearcherRemoveRangeMixed(t *testing.T) {
	t.Parallel()

	testBinarySearcherRemoveRange(t, 10000, "192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24", "2001:db8::/112", "::/64", "1::/58")
}

func TestInsertRangeWithLastAlready(t *testing.T) {
	t.Parallel()

	server, _, err := setup(false)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

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

	ip4, ip6, ip6r, err := server.Count()
	if err != nil {
		t.Error(err)
	}

	if ip4 != 257 || ip6 != 0 || ip6r != 0 {
		t.Errorf("blocklist returned invalid count, expected (257, 0, 0), got (%d, %d, %d)", ip4, ip6, ip6r)
	}
}

func TestRemoveRangeNonExistAtEnd(t *testing.T) {
	t.Parallel()

	server, _, err := setup(false)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

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

	ip4, ip6, ip6r, err := server.Count()
	if err != nil {
		t.Error(err)
	}

	if ip4 != 1 || ip6 != 0 || ip6r != 0 {
		t.Errorf("blocklist returned invalid count, expected (1, 0, 0), got (%d, %d, %d)", ip4, ip6, ip6r)
	}
}

func TestAddLargeRange(t *testing.T) {
	t.Parallel()

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

func TestServerRWLocker(t *testing.T) {
	t.Parallel()

	server, _, err := setup(false)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	header := (*shmHeader)(unsafe.Pointer(&server.data[0]))
	lock := header.rwLocker()
	lock.Lock()
	lock.Unlock()
}

func TestClientRWLocker(t *testing.T) {
	t.Parallel()

	server, client, err := setup(true)
	if err != nil {
		t.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()
	defer client.Close()

	header := (*shmHeader)(unsafe.Pointer(&client.data[0]))
	lock := header.rwLocker()
	lock.RLock()
	lock.RUnlock()
}

func TestOpenNonExist(t *testing.T) {
	t.Parallel()

	client, err := Open("/go-test-bogus-name")
	if err == nil {
		client.Close()
	}

	if !os.IsNotExist(err) {
		t.Error("Open did not return non exist error for bogus name")
	}
}

func TestNewOpenEmptyName(t *testing.T) {
	t.Parallel()

	client, err := Open("")
	if err == nil {
		client.Close()
	}

	if err != syscall.EINVAL {
		t.Error("Open did not return EINVAL for empty name")
	}

	server, err := New("", 0)
	if err == nil {
		server.Close()
		server.Unlink()
	}

	if err != syscall.EINVAL {
		t.Error("New did not return EINVAL for empty name")
	}
}

func TestUnlinkEmptyName(t *testing.T) {
	t.Parallel()

	if err := Unlink(""); err == nil {
		t.Errorf("Unlink did not return error for empty name")
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
	t.Parallel()

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
	t.Parallel()

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
}

func TestInvalidAddr(t *testing.T) {
	t.Parallel()

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

func benchmarkInsert(b *testing.B, addr string, extra int, batch bool) {
	server, _, err := setup(false)
	if err != nil {
		b.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	if batch {
		if err = server.Batch(); err != nil {
			b.Error(err)
		}
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
	benchmarkInsert(b, "192.0.2.0", 0, false)
}

func BenchmarkInsertIP6NoSearch(b *testing.B) {
	benchmarkInsert(b, "2001:db8::", 0, false)
}

func BenchmarkInsertIP4(b *testing.B) {
	benchmarkInsert(b, "192.0.2.0", 10000, false)
}

func BenchmarkInsertIP6(b *testing.B) {
	benchmarkInsert(b, "2001:db8::", 10000, false)
}

func BenchmarkInsertBatchIP4NoSearch(b *testing.B) {
	benchmarkInsert(b, "192.0.2.0", 0, true)
}

func BenchmarkInsertBatchIP6NoSearch(b *testing.B) {
	benchmarkInsert(b, "2001:db8::", 0, true)
}

func BenchmarkInsertBatchIP4(b *testing.B) {
	benchmarkInsert(b, "192.0.2.0", 10000, true)
}

func BenchmarkInsertBatchIP6(b *testing.B) {
	benchmarkInsert(b, "2001:db8::", 10000, true)
}

func benchmarkRemove(b *testing.B, addr string, extra int, batch bool) {
	server, _, err := setup(false)
	if err != nil {
		b.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

	if batch {
		if err = server.Batch(); err != nil {
			b.Error(err)
		}
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
	benchmarkRemove(b, "192.0.2.0", 0, false)
}

func BenchmarkRemoveIP6NoSearch(b *testing.B) {
	benchmarkRemove(b, "2001:db8::", 0, false)
}

func BenchmarkRemoveIP4(b *testing.B) {
	benchmarkRemove(b, "192.0.2.0", 10000, false)
}

func BenchmarkRemoveIP6(b *testing.B) {
	benchmarkRemove(b, "2001:db8::", 10000, false)
}

func BenchmarkRemoveBatchIP4NoSearch(b *testing.B) {
	benchmarkRemove(b, "192.0.2.0", 0, true)
}

func BenchmarkRemoveBatchIP6NoSearch(b *testing.B) {
	benchmarkRemove(b, "2001:db8::", 0, true)
}

func BenchmarkRemoveBatchIP4(b *testing.B) {
	benchmarkRemove(b, "192.0.2.0", 10000, true)
}

func BenchmarkRemoveBatchIP6(b *testing.B) {
	benchmarkRemove(b, "2001:db8::", 10000, true)
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
	benchmarkContains(b, "192.0.2.0", 10000)
}

func BenchmarkContainsIP6(b *testing.B) {
	benchmarkContains(b, "2001:db8::", 10000)
}

func benchmarkCommit(b *testing.B, extra int) {
	server, _, err := setup(false)
	if err != nil {
		b.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()

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

	for i := 0; i < b.N; i++ {
		b.StopTimer()

		if err = server.Batch(); err != nil {
			b.Error(err)
			continue
		}

		b.StartTimer()

		if err = server.Commit(); err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkCommitEmpty(b *testing.B) {
	benchmarkCommit(b, 0)
}

func BenchmarkCommit(b *testing.B) {
	benchmarkCommit(b, 10000/3)
}

func BenchmarkClientRemap(b *testing.B) {
	server, client, err := setup(true)
	if err != nil {
		b.Fatal(err)
	}

	defer server.Unlink()
	defer server.Close()
	defer client.Close()

	header := (*shmHeader)(unsafe.Pointer(&client.data[0]))
	header.rwLocker().RLock()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err = client.remap(true); err != nil {
			b.Fatal(err)
		}
	}

	b.StopTimer()

	header = (*shmHeader)(unsafe.Pointer(&client.data[0]))
	header.rwLocker().RUnlock()
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

func BenchmarkInsertRangeIP4NoSearch24(b *testing.B) {
	benchmarkInsertRemoveRange(b, true, "192.0.2.0/24", 0)
}

func BenchmarkInsertRangeIP6NoSearch116(b *testing.B) {
	benchmarkInsertRemoveRange(b, true, "2001:db8::/116", 0)
}

func BenchmarkInsertRangeIP6RouteNoSearch54(b *testing.B) {
	benchmarkInsertRemoveRange(b, true, "2001:db8::/54", 0)
}

func BenchmarkInsertRangeIP424(b *testing.B) {
	benchmarkInsertRemoveRange(b, true, "192.0.2.0/24", 10000)
}

func BenchmarkInsertRangeIP6116(b *testing.B) {
	benchmarkInsertRemoveRange(b, true, "2001:db8::/116", 10000)
}

func BenchmarkInsertRangeIP6Route54(b *testing.B) {
	benchmarkInsertRemoveRange(b, true, "2001:db8::/54", 10000)
}

func BenchmarkRemoveRangeIP4NoSearch24(b *testing.B) {
	benchmarkInsertRemoveRange(b, false, "192.0.2.0/24", 0)
}

func BenchmarkRemoveRangeIP6NoSearch116(b *testing.B) {
	benchmarkInsertRemoveRange(b, false, "2001:db8::/116", 0)
}

func BenchmarkRemoveRangeIP6RouteNoSearch54(b *testing.B) {
	benchmarkInsertRemoveRange(b, false, "2001:db8::/54", 0)
}

func BenchmarkRemoveRangeIP424(b *testing.B) {
	benchmarkInsertRemoveRange(b, false, "192.0.2.0/24", 10000)
}

func BenchmarkRemoveRangeIP6116(b *testing.B) {
	benchmarkInsertRemoveRange(b, false, "2001:db8::/116", 10000)
}

func BenchmarkRemoveRangeIP6Route54(b *testing.B) {
	benchmarkInsertRemoveRange(b, false, "2001:db8::/54", 10000)
}
