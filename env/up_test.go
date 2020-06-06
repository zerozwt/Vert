package env

import (
	"errors"
	"fmt"
	"hash/crc32"
	"math/rand"
	"net/http"
	"runtime"
	"testing"
)

func testRoundRobin(domain string, expect []string) error {
	for _, addr := range expect {
		if tmp := UpstreamAddr(domain, nil); tmp != addr {
			msg := fmt.Sprintf("round robin %s failed: expect=%s answer=%s", domain, addr, tmp)
			return errors.New(msg)
		}
	}
	return nil
}

func TestUpstream(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	raw := map[string][]string{
		" aa  ": {
			"A",
			"B weight=2",
			"C",
		},
		" bb round_robin ": {
			"A",
			"B",
			"C",
		},
		" cc random ": {
			"A",
			"B",
			"C",
		},
		" dd client_hash ": {
			"A",
			"B",
			"C",
		},
	}

	if err := AddUpsteam(raw); err != nil {
		t.Errorf("build upstream failed: %v", err)
		return
	}

	if err := testRoundRobin("aa", []string{"B", "B", "C", "A", "B", "B", "C", "A"}); err != nil {
		t.Error(err)
		return
	}

	if err := testRoundRobin("bb", []string{"B", "C", "A", "B", "C", "A"}); err != nil {
		t.Error(err)
		return
	}

	for i := 0; i < 100; i++ {
		client_addr := make([]byte, 16)
		rand.Read(client_addr)
		addr_1 := UpstreamAddr("dd", &http.Request{RemoteAddr: string(client_addr)})
		addr_2 := UpstreamAddr("dd", &http.Request{RemoteAddr: string(client_addr)})

		if addr_1 != addr_2 {
			t.Errorf("client_hash returned different answer with same client_addr")
			return
		}
	}

	crc32.ChecksumIEEE([]byte{})
}
