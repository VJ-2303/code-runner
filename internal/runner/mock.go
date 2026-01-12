package runner

import "context"

type MockRunner struct{}

func (m *MockRunner) Run(ctx context.Context, code string, language string) (*ExecuteResult, error) {
	return &ExecuteResult{
		Output: "Mock Output: " + code,
		Error:  "",
	}, nil
}
