package env

// Var - type Var.
type Var = string

// THIS CONST is Enum of all available environments
// IT USED IN CI/CD(Gitlab)
// It's passed to program by set -X AppEnv arg in build Docker image step
// nolint golint
const (
	Name = "price_monitor"

	Dev   Var = "dev"
	Test  Var = "test"
	Stage Var = "stage"
	Prod  Var = "prod"
)
