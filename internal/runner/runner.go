package runner

import "context"

type ExecuteResult struct {
	Output string `json:"output"`
	Error  string `json:"error"`
}

type Runner interface {
	Run(ctx context.Context, code string, language string) (*ExecuteResult, error)
}
