package workflow

type ExecuteActivity struct {
	Name string `json:"name"`

	//todo: options
	Args []interface{} `json:"arguments"`
}

type CompleteWorkflow struct {
	Result []interface{} `json:"result"`
}
