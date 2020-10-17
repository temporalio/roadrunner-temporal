package temporal

import "github.com/spiral/roadrunner/v2"

type Config struct {
	Address    string
	Namespace  string
	Activities *roadrunner.Config
}
