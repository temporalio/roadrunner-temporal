package workflow

import "time"

type ExecuteActivity struct {
	Name string `json:"name"`

	//todo: options
	Args []interface{} `json:"arguments"`
}

type CompleteWorkflow struct {
	Result []interface{} `json:"result"`
}

type NewTimer struct {
	Milliseconds int `json:"ms"`
}

func (t NewTimer) ToDuration() time.Duration {
	return time.Millisecond * time.Duration(t.Milliseconds)
}
