package tools

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/StevenZack/tools/strToolkit"
)

// parse `00:01:20.480363` into seconds
func ParseDurationSeconds(s string) (int, error) {
	s = strToolkit.SubBeforeLast(s, ".", s)
	ss := strings.Split(s, ":")
	if len(ss) != 3 {
		return 0, fmt.Errorf("Invalid duration format: %s", s)
	}
	var seconds int
	for i, v := range ss {
		num, e := strconv.Atoi(v)
		if e != nil {
			return 0, fmt.Errorf("Invalid duration format: %s", s)
		}
		switch i {
		case 0:
			seconds += num * 3600
		case 1:
			seconds += num * 60
		case 2:
			seconds += num
		}
	}
	return seconds, nil
}
