package travel

import (
	"hash/fnv"
	"strings"
)

func seed(parts ...string) uint32 {
	h := fnv.New32a()
	for _, p := range parts {
		h.Write([]byte(strings.ToLower(p)))
	}
	return h.Sum32()
}
