package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

type LanguageConfig struct {
	Image    string
	FileName string
	Command  []string
}

type DockerRunner struct {
	config map[string]LanguageConfig
}

func NewDockerRunner() *DockerRunner {
	return &DockerRunner{
		config: map[string]LanguageConfig{
			"python": {
				Image:    "python:alpine",
				FileName: "main.py",
				Command:  []string{"python", "/app/main.py"},
			},
			"ruby": {
				Image:    "ruby:alpine",
				FileName: "main.rb",
				Command:  []string{"ruby", "/app/main.rb"},
			},
			"javascript": {
				Image:    "node:alpine",
				FileName: "index.js",
				Command:  []string{"node", "index.js"},
			},
		},
	}
}

func (dr *DockerRunner) Run(ctx context.Context, code, language string) (*ExecuteResult, error) {
	config, ok := dr.config[language]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}
	tmpDir, err := os.MkdirTemp("", "runner-*")
	if err != nil {
		return nil, fmt.Errorf("Failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	hostFilePath := filepath.Join(tmpDir, config.FileName)
	if err := os.WriteFile(hostFilePath, []byte(code), 0o644); err != nil {
		return nil, fmt.Errorf("failed to write code to file: %w", err)
	}
	dockerArgs := []string{
		"run",
		"--rm",
		"--network", "none",
		"--memory", "128m",
		"--cpus", "0.5",
		"-v", fmt.Sprintf("%s:/app/%s", hostFilePath, config.FileName),
		"-w", "/app",
		config.Image,
	}
	dockerArgs = append(dockerArgs, config.Command...)

	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	result := ExecuteResult{
		Output: stdout.String(),
		Error:  stderr.String(),
	}

	if ctx.Err() == context.DeadlineExceeded {
		result.Error = "Execution timed out"
		return &result, nil
	}

	if err != nil {
		if result.Error == "" {
			result.Error = fmt.Sprintf("Execution failed: %v", err)
		}
	}
	return &result, nil
}
