package serialize

import "encoding/json"

// Serialize serialize
func Serialize(dict *map[string]interface{}) ([]byte, error) {
	return json.Marshal(dict)
}

// Deserialize deserialize
func Deserialize(data []byte, dict *map[string]interface{}) error {
	return json.Unmarshal(data, dict)
}
