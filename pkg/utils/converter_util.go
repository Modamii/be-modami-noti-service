package utils

import "be-modami-no-service/pkg/contract"

// resolveRecipients returns user IDs from extra.To or payload (e.g. audience_ids in do[0].data).
func ResolveRecipients(e *contract.NotificationEvent) []string {
	if e.Extra != nil && e.Extra.To != nil {
		return SliceFromInterface(e.Extra.To)
	}
	if len(e.Payload.Do) == 0 {
		return nil
	}
	data := e.Payload.Do[0].Data
	if data == nil {
		return nil
	}
	if ids, ok := data["audience_ids"]; ok {
		return SliceFromInterface(ids)
	}
	return nil
}

func GetStr(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

func SliceFromInterface(v interface{}) []string {
	switch x := v.(type) {
	case []string:
		return x
	case []interface{}:
		var out []string
		for _, i := range x {
			if s, ok := i.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
