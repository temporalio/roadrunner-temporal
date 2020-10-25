package roadrunner_temporal

type Command struct {
	// Command name.
	Command string `json:"command"`

	// ID represents command when
	ID uint64 `json:"id"`

	// Params represent custom command params.
	Params interface{} `json:"params"`
}
