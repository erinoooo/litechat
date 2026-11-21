package db

import "encoding/json"

func encodeImages(imgs []string) string {
	if len(imgs) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(imgs)
	return string(b)
}

func decodeImages(s string) []string {
	out := []string{}
	_ = json.Unmarshal([]byte(s), &out)
	return out
}
