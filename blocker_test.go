// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	mrand "math/rand"
	"net"
	"os"
	"testing"
)

var rand *mrand.Rand

func init() {
	var seed [8]byte

	if _, err := crand.Read(seed[:]); err != nil {
		panic(err)
	}

	seedInt := int64(binary.LittleEndian.Uint64(seed[:]))
	rand = mrand.New(mrand.NewSource(seedInt))
}

func setup(withClient bool) (*Server, *Client, error) {
	name := fmt.Sprintf("/go-test-%d", rand.Int())

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
		t.Error(err)
		return
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
		t.Error(err)
		return
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
			t.Error(err)
		}

		if err = server.InsertRange(ip, ipnet); err != nil {
			t.Error(err)
			return
		}
	}

	for _, addr := range addrs {
		has, err := client.Contains(net.ParseIP(addr))
		if err != nil {
			t.Error(err)
			continue
		}

		if !has {
			t.Error("blocklist does not contain entry after insert")
			continue
		}
	}

	for _, iprange := range ipranges {
		ip, ipnet, err := net.ParseCIDR(iprange)
		if err != nil {
			t.Error(err)
		}

		if err = server.RemoveRange(ip, ipnet); err != nil {
			t.Error(err)
			return
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

	testRange(t, []string{"192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24", "2001:db8::/112", "2001:db8::/112", "::/64", "1::/58"}, "192.0.2.0", "192.0.2.1", "198.51.100.0", "198.51.100.128", "203.0.113.0", "203.0.113.255", "2001:db8::", "2001:db8::1", "2001:db8::f", "::", "::1", "::f", "::dead:beef", "1::", "1::1", "1::f", "1::dead:beef")
}

func testClear(t *testing.T, addrs ...string) {
	server, client, err := setup(true)
	if err != nil {
		t.Error(err)
		return
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
	}

	if err = server.Clear(); err != nil {
		t.Error(err)
		return
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

func TestBatch(t *testing.T) {
	t.Parallel()

	server, _, err := setup(false)
	if err != nil {
		t.Error(err)
		return
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

func TestBlockerClose(t *testing.T) {
	t.Parallel()

	server, _, err := setup(false)
	if err != nil {
		t.Error(err)
		return
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
		t.Error(err)
		return
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
		t.Error(err)
		return
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
}

func TestServerCount(t *testing.T) {
	t.Parallel()

	server, _, err := setup(false)
	if err != nil {
		t.Error(err)
		return
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
}

func TestClientCount(t *testing.T) {
	t.Parallel()

	server, client, err := setup(true)
	if err != nil {
		t.Error(err)
		return
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
}

func BenchmarkNew(b *testing.B) {
	name := fmt.Sprintf("/go-test-%d", rand.Int())

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
		b.Error(err)
		return
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
		b.Error(err)
		return
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
		mrand.Read(extraIP)

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
		b.Error(err)
		return
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
		mrand.Read(extraIP)

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
		b.Error(err)
		return
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
		mrand.Read(extraIP)

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
			return
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
		b.Error(err)
		return
	}

	defer server.Unlink()
	defer server.Close()

	extraIP := make(net.IP, net.IPv6len)

	for i := 0; i < extra; i++ {
		mrand.Read(extraIP)

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
		b.Error(err)
		return
	}

	defer server.Unlink()
	defer server.Close()
	defer client.Close()

	lock := client.rwlockerForTest()
	lock.RLock()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err = client.remap(true); err != nil {
			b.Error(err)
		}
	}

	b.StopTimer()

	lock = client.rwlockerForTest()
	lock.RUnlock()
}
