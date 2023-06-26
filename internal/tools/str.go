package tools

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/StevenZack/tools/numToolkit"
)

func Jsonify(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func GenerateID() string {
	return strconv.FormatInt(time.Now().UnixNano()+numToolkit.Rand63n(1000), 10)
}
