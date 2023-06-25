package tools

import "encoding/json"

func Jsonify(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
