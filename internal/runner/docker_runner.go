package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type LanguageConfig struct {
	Image    string
	FileName string
	Command  []string
}

type DockerRunner struct {
	client *client.Client
	logger *slog.Logger
	pool   *ContainerPool
	config map[string]LanguageConfig
}

func NewDockerRunner(logger *slog.Logger, poolSize int) (*DockerRunner, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	config := map[string]LanguageConfig{
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
			Command:  []string{"node", "/app/index.js"},
		},
	}
	pool, err := NewContainerPool(cli, logger, config, poolSize)
	if err != nil {
		cli.Close()
		return nil, fmt.Errorf("failed to create container pool: %w", err)
	}
	return &DockerRunner{
		client: cli,
		logger: logger,
		pool:   pool,
		config: config,
	}, nil
}

func (dr *DockerRunner) Run(ctx context.Context, code, language, stdin string) (*ExecuteResult, error) {
	langCfg, ok := dr.config[language]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	containerID, err := dr.pool.Get(language)
	if err != nil {
		return nil, fmt.Errorf("failed to get container from the pool: %w", err)
	}
	defer dr.pool.Recycle(language, containerID)

	shellScript := fmt.Sprintf(
		`mkdir -p /app && cat > /app/%s << '__CODE_EOF__'
%s
__CODE_EOF__
%s`,
		langCfg.FileName, code, strings.Join(langCfg.Command, " "),
	)

	execCfg := container.ExecOptions{
		Cmd:          []string{"sh", "-c", shellScript},
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
	}
	execResp, err := dr.client.ContainerExecCreate(ctx, containerID, execCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec: %w", err)
	}

	attachResp, err := dr.client.ContainerExecAttach(ctx, execResp.ID, container.ExecStartOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer attachResp.Close()

	go func() {
		if stdin != "" {
			io.Copy(attachResp.Conn, strings.NewReader(stdin))
		}
		attachResp.CloseWrite()
	}()

	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, attachResp.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read exec output: %w", err)
	}

	execInspect, err := dr.client.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect exec: %w", err)
	}

	result := &ExecuteResult{
		Output: stdout.String(),
		Error:  stderr.String(),
	}

	if ctx.Err() == context.DeadlineExceeded {
		result.Error = "Execution timed out"
		return result, nil
	}

	if execInspect.ExitCode != 0 && result.Error == "" {
		result.Error = fmt.Sprintf("process exited with code %d", execInspect.ExitCode)
	}

	return result, nil
}

func (dr *DockerRunner) Close() {
	dr.pool.Close()
	dr.client.Close()
}
