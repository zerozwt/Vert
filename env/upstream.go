package env

import (
	"errors"
	"hash/crc32"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
)

const us_round_robin int = 1
const us_random int = 2
const us_client_hash int = 3

type upstreamItem struct {
	addr   string
	weight int
}

type upstreamGroup struct {
	strategy int

	entries []upstreamItem

	mod          uint32
	curr_idx     uint32
	total_weight uint32
}

var gUpstreamMap map[string]*upstreamGroup = make(map[string]*upstreamGroup)

var matchUpstreamName *regexp.Regexp = regexp.MustCompile(`^([^ \t]+)([ \t]+[^ \t]+)?$`)
var matchUpstreamEntry *regexp.Regexp = regexp.MustCompile(`^([^ \t]+)([ \t]+weight=[0-9]+)?$`)

var strategy2int map[string]int = map[string]int{
	"round_robin": us_round_robin,
	"random":      us_random,
	"client_hash": us_client_hash,
}

func AddUpsteam(conf map[string][]string) error {
	for name, entry_list := range conf {
		key := strings.Trim(name, " \r\n\t")
		match := matchUpstreamName.FindAllSubmatchIndex([]byte(key), -1)
		if len(match) == 0 {
			return errors.New("Malformed upstream name: " + key)
		}

		if len(entry_list) == 0 {
			return errors.New("No address in upstream " + key)
		}

		domain := key[match[0][2]:match[0][3]]
		strategy := "round_robin"

		if match[0][4] != match[0][5] {
			strategy = strings.Trim(key[match[0][4]:match[0][5]], " \r\n\t")
		}

		if strategy != "round_robin" && strategy != "random" && strategy != "client_hash" {
			return errors.New("Invalid strategy '" + strategy + "' for upstream " + domain)
		}

		group := &upstreamGroup{
			strategy: strategy2int[strategy],
			entries:  make([]upstreamItem, 0),
		}

		for _, entry_str := range entry_list {
			entry_bytes := []byte(strings.Trim(entry_str, " \r\n\t"))
			match := matchUpstreamEntry.FindAllSubmatchIndex(entry_bytes, -1)
			if len(match) == 0 {
				return errors.New("Malformed address for " + domain + " : " + entry_str)
			}

			addr := string(entry_bytes[match[0][2]:match[0][3]])
			weight := 1

			if match[0][4] != match[0][5] {
				weight_str := string(entry_bytes[match[0][4]:match[0][5]])
				eq_idx := strings.Index(weight_str, "=")

				var err error
				weight, err = strconv.Atoi(weight_str[eq_idx+1:])
				if err != nil {
					return err
				}
			}

			if weight < 1 {
				return errors.New("upstream addr " + addr + " weight cannot be less than 1")
			}

			group.entries = append(group.entries, upstreamItem{addr: addr, weight: weight})
			group.total_weight += uint32(weight)
		}

		gUpstreamMap[domain] = group
	}
	return nil
}

func UpstreamAddr(domain string, req *http.Request) string {
	group, ok := gUpstreamMap[domain]
	if !ok {
		return ""
	}

	return group.getAddr(req)
}

func (self *upstreamGroup) getAddr(req *http.Request) string {
	switch self.strategy {
	case us_round_robin:
		return self.getAddr_RoundRobin()
	case us_random:
		return self.getAddr_Random()
	case us_client_hash:
		return self.getAddr_ClientHash(req)
	}
	return ""
}

func (self *upstreamGroup) getAddr_RoundRobin() string {
	idx := atomic.AddUint32(&(self.curr_idx), 1)
	if idx >= self.total_weight {
		self.tryMod()
		idx = idx % self.total_weight
	}
	for _, item := range self.entries {
		if idx < uint32(item.weight) {
			return item.addr
		}
		idx -= uint32(item.weight)
	}
	return ""
}

func (self *upstreamGroup) tryMod() {
	if atomic.CompareAndSwapUint32(&(self.mod), 0, 1) {
		go func() {
			defer atomic.StoreUint32(&(self.mod), 0)
			curr := atomic.LoadUint32(&(self.curr_idx))
			for !atomic.CompareAndSwapUint32(&(self.curr_idx), curr, curr%self.total_weight) {
				curr = atomic.LoadUint32(&(self.curr_idx))
			}
		}()
	}
}

func (self *upstreamGroup) getAddr_Random() string {
	return self.entries[rand.Intn(len(self.entries))].addr
}

func (self *upstreamGroup) getAddr_ClientHash(req *http.Request) string {
	client_addr := req.RemoteAddr
	port_idx := strings.Index(client_addr, ":")

	if port_idx >= 0 {
		client_addr = client_addr[:port_idx]
	}

	idx := crc32.ChecksumIEEE([]byte(client_addr)) % self.total_weight

	return self.entries[idx].addr
}
