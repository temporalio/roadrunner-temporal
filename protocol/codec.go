package roadrunner_temporal

type Codec interface {
	Execute(e Endpoint, ctx Context, msg ...Message) ([]Message, error)
}
