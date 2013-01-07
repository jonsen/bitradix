package bitradix

import (
	"net"
	"reflect"
	"testing"
)

func TestInsert(t *testing.T) {
	tests := map[uint32]uint32{
		0x08: 2012,
		0x04: 2010,
		0x09: 2013,
	}
	r := New()
	for key, value := range tests {
		if x := r.Insert(key, 4, value); x.Value != value {
			t.Logf("Expected %d, got %d for %d (node type %v)\n", value, x.Value, key, x.Internal())
			t.Fail()
		}
	}
}

func TestInsertIdempotent(t *testing.T) {
	r := New()
	r.Insert(0x08, 4, 2012)
	r.Insert(0x08, 4, 2013)
	if x := r.Find(0x08, 4); x.Value != 2013 {
		t.Logf("Expected %d, got %d for %d\n", 2013, x.Value, 0x08)
		t.Fail()
	}
}

func TestFindExact(t *testing.T) {
	tests := map[uint32]uint32{
		0x80000000: 2012,
		0x40000000: 2010,
		0x90000000: 2013,
	}
	r := New()
	for k, v := range tests {
		t.Logf("Tree after insert of %032b (%x %d)\n", k, k, k)
		r.Insert(k, 5, v)
		r.Do(func(r1 *Radix, i int) { t.Logf("(%2d): %032b/%d -> %d\n", i, r1.key, r1.bits, r1.Value) })
	}
	for k, v := range tests {
		if x := r.Find(k, 5); x.Value != v {
			t.Logf("Expected %d, got %d for %d (node type %v)\n", v, x.Value, k, x.Internal())
			t.Fail()
		}
	}
}

// Test with "real-life" ip addresses
func ipToUint(t *testing.T, n *net.IPNet) (i uint32, mask int) {
	ip := n.IP.To4()
	fv := reflect.ValueOf(&i).Elem()
	fv.SetUint(uint64(uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[+3])))
	mask, _ = n.Mask.Size()
	return
}

func addRoute(t *testing.T, r *Radix, s string, asn uint32) {
	_, ipnet, _ := net.ParseCIDR(s)
	net, mask := ipToUint(t, ipnet)
	t.Logf("Route %s (%032b), AS %d\n", s, net, asn)
	r.Insert(net, mask, asn)
}

func findRoute(t *testing.T, r *Radix, s string) uint32 {
	_, ipnet, _ := net.ParseCIDR(s)
	net, mask := ipToUint(t, ipnet)
	t.Logf("Search %18s %032b/%d\n", s, net, mask)
	node := r.Find(net, mask)
	return node.Value
}

func TestFindIP(t *testing.T) {
	r := New()
	// not a map to have influence on the order
	addRoute(t, r, "10.0.0.2/8", 10)
	addRoute(t, r, "10.20.0.0/14", 20)
	addRoute(t, r, "10.21.0.0/16", 21)
	addRoute(t, r, "192.168.0.0/16", 192)
	addRoute(t, r, "192.168.2.0/24", 1922)

	testips := map[string]uint32{
		"10.20.1.2/32":   20,
		"10.22.1.2/32":   20,
		"10.19.0.1/32":   10,
		"10.21.0.1/32":   21,
		"192.168.2.3/32": 1922,
		"230.0.0.1/32":   0,
	}

	for ip, asn := range testips {
		if x := findRoute(t, r, ip); asn != x {
			t.Logf("Expected %d, got %d for %s\n", asn, x, ip)
			t.Fail()
		}
	}
}

func TestFindIPShort(t *testing.T) {
	r := New()
	// not a map to have influence on the inserting order
	addRoute(t, r, "10.0.0.2/8", 10)
	addRoute(t, r, "10.0.0.0/14", 11)
	addRoute(t, r, "10.20.0.0/14", 20)

	r.Do(func(r1 *Radix, i int) { t.Logf("(%2d): %032b/%d -> %d\n", i, r1.key, r1.bits, r1.Value) })

	testips := map[string]uint32{
		"10.20.1.2/32": 20,
		"10.19.0.1/32": 10,
		"10.0.0.2/32": 10,
		"10.1.0.1/32": 10,
	}

	for ip, asn := range testips {
		if x := findRoute(t, r, ip); asn != x {
			t.Logf("Expected %d, got %d for %s\n", asn, x, ip)
			t.Fail()
		}
	}
}

type bittest struct {
	value uint32
	bit   int
}

func TestBitK(t *testing.T) {
	tests := map[bittest]byte{
		bittest{0x40, 0}: 0,
		bittest{0x40, 6}: 1,
	}
	for test, expected := range tests {
		if x := bitK(test.value, test.bit); x != expected {
			t.Logf("Expected %d for %032b (bit #%d), got %d\n", expected, test.value, test.bit, x)
			t.Fail()
		}
	}
}

func TestQueue(t *testing.T) {
	q := make(queue, 0)
	r := New()
	r.Value = 10

	q.Push(&node{r, -1})
	if r1 := q.Pop(); r1.Value != 10 {
		t.Logf("Expected %d, got %d\n", 10, r.Value)
		t.Fail()
	}
	if r1 := q.Pop(); r1 != nil {
		t.Logf("Expected nil, got %d\n", r.Value)
		t.Fail()
	}
}

func TestQueue2(t *testing.T) {
	q := make(queue, 0)
	tests := []uint32{20, 30, 40}
	for _, val := range tests {
		q.Push(&node{&Radix{Value: val}, -1})
	}
	for _, val := range tests {
		x := q.Pop()
		if x == nil {
			t.Logf("Expected non-nil, got nil\n")
			t.Fail()
			continue
		}
		if x.Radix.Value != val {
			t.Logf("Expected %d, got %d\n", val, x.Radix.Value)
			t.Fail()
		}
	}
	if x := q.Pop(); x != nil {
		t.Logf("Expected nil, got %d\n", x.Radix.Value)
		t.Fail()
	}
	// Push and pop again, see if that works too
	for _, val := range tests {
		q.Push(&node{&Radix{Value: val}, -1})
	}
	for _, val := range tests {
		x := q.Pop()
		if x == nil {
			t.Logf("Expected non-nil, got nil after emptying\n")
			t.Fail()
			continue
		}
		if x.Radix.Value != val {
			t.Logf("Expected %d, got %d\n", val, x.Radix.Value)
			t.Fail()
		}
	}
}
