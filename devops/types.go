package devops

import (
	"encoding/json"
	"fmt"
)

// StringOrNumber unmarshals JSON string or number into a string.
type StringOrNumber string

func (s *StringOrNumber) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*s = StringOrNumber(str)
		return nil
	}
	var num json.Number
	if err := json.Unmarshal(data, &num); err != nil {
		return fmt.Errorf("cannot unmarshal %q as string or number", data)
	}
	*s = StringOrNumber(num.String())
	return nil
}

// APIResponse is the common DevOps JSON envelope.
type APIResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func (r *APIResponse) IsSuccess() bool { return r.Code == 0 }

func (r *APIResponse) WithData(v any) error {
	if len(r.Data) == 0 || string(r.Data) == "null" {
		return nil
	}
	return json.Unmarshal(r.Data, v)
}

// Authority is a permission item from login.
type Authority struct {
	ID         StringOrNumber `json:"id"`
	ParentID   StringOrNumber `json:"parentId"`
	MenuName   string         `json:"menuName"`
	Permission string         `json:"permission"`
	MultiEnv   int            `json:"multiEnv"`
	PageHref   string         `json:"pageHref"`
	Envs       []string       `json:"envs"`
	Child      any            `json:"child"`
}
