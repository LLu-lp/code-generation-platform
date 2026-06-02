// Package utils — JSON 序列化/反序列化工具
//
// ToJSON：对象 → JSON 字符串
// FromJSON：JSON 字符串 → 对象
// ToJSONBytes：对象 → JSON 字节切片
package utils

import (
	"encoding/json"
	"log"
)

// ToJSON 对象转 JSON 字符串
func ToJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("[JSON] 序列化失败: %v", err)
		return "{}"
	}
	return string(data)
}

// FromJSON JSON 字符串转对象
func FromJSON(data string, v interface{}) error {
	return json.Unmarshal([]byte(data), v)
}

// ToJSONBytes 对象转 JSON 字节
func ToJSONBytes(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return data
}
