package logging

import (
	"encoding/json"
	"fmt"
)

type JsonFormatter struct{}

func NewJsonFormatter() *JsonFormatter {
	return &JsonFormatter{}
}

func (j JsonFormatter) Format(record *LogRecord) []byte {
	propertiesCount := len(record.Properties) / 2

	data := make(map[string]interface{}, 3+propertiesCount)
	data["timestamp"] = record.Timestamp
	data["level"] = record.Level.String()
	data["message"] = record.Message
	for i := 0; i < propertiesCount; i++ {
		key := fmt.Sprint(record.Properties[i*2])
		value := fmt.Sprint(record.Properties[i*2+1])
		data[key] = value
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return []byte(`{"error":"json marshal failed"}`)
	}

	return jsonBytes
}
