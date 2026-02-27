package helpers

import (
	"encoding/json"
	"sort"
)

type Usage struct {
	Quantity int    `json:"quantity"`
	Type     string `json:"type"`
	Unit     string `json:"unit"`
}

type MeteringMsg struct {
	CloudID    string                 `json:"cloud_id"`
	FolderID   string                 `json:"folder_id"`
	ResourceID string                 `json:"resource_id"`
	Schema     string                 `json:"schema"`
	Tags       map[string]interface{} `json:"tags"`
	Labels     map[string]interface{} `json:"labels"`
	Usage      Usage                  `json:"usage"`
	Version    string                 `json:"version"`
}

func reduceMeteringData(msgs []MeteringMsg) []MeteringMsg {
	type agg struct {
		msg MeteringMsg
		qty int
	}

	byKey := make(map[string]*agg, len(msgs))
	for _, msg := range msgs {
		keyMsg := msg
		keyMsg.Usage.Quantity = 0
		keyJSON, _ := json.Marshal(keyMsg)
		key := string(keyJSON)

		entry, ok := byKey[key]
		if !ok {
			entry = &agg{msg: keyMsg}
			byKey[key] = entry
		}
		entry.qty += msg.Usage.Quantity
	}

	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]MeteringMsg, 0, len(keys))
	for _, key := range keys {
		entry := byKey[key]
		entry.msg.Usage.Quantity = entry.qty
		result = append(result, entry.msg)
	}
	return result
}
