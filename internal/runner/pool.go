package runner

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

type ContainerPool struct {
	client *client.Client
	logger *slog.Logger
	config map[string]LanguageConfig

	pools    map[string]chan string
	poolSise int

	mu       sync.Mutex
	shutdown bool
}

func NewContainerPool(cli *client.Client, logger *slog.Logger, config map[string]LanguageConfig, poolSize int) (*ContainerPool, error) {
	pool := &ContainerPool{
		client:   cli,
		logger:   logger,
		config:   config,
		pools:    make(map[string]chan string),
		poolSise: poolSize,
	}

	for lang := range config {
		pool.pools[lang] = make(chan string, poolSize)
	}

	if err := pool.warmAll(); err != nil {
		return nil, fmt.Errorf("failed to warm container pool: %w", err)
	}
	return pool, nil
}

func (p *ContainerPool) warmAll() error {
	for lang := range p.config {
		for i := 0; i < p.poolSise; i++ {
			id, err := p.createWarmContainer(lang)
			if err != nil {
				return fmt.Errorf("failed to warm %s container %d: %w", lang, i, err)
			}
			p.pools[lang] <- id
			p.logger.Info("warmed container", "language", lang, "container_id", id[:12])
		}
	}
	return nil
}

func (p *ContainerPool) createWarmContainer(language string) (string, error) {
	langCfg := p.config[language]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	containerCfg := &container.Config{
		Image: langCfg.Image,
		Cmd:   []string{"sleep", "infinity"},
		User:  "nobody",

		Labels: map[string]string{
			"code-runner.pool":     "true",
			"code-runner.language": language,
		},
	}
	hostCfg := &container.HostConfig{
		Resources: container.Resources{
			Memory:    128 * 1024 * 1024,
			NanoCPUs:  500_000_000,
			PidsLimit: int64Ptr(64),
		},
		NetworkMode:    "none",
		ReadonlyRootfs: true,

		CapDrop: []string{"ALL"},

		Tmpfs: map[string]string{
			"/tmp": "rw,noexec,nosuid,size=16m",
			"/app": "rw,noexec,nosuid,size=8m",
		},

		SecurityOpt: []string{"no-new-privileges"},
	}
	resp, err := p.client.ContainerCreate(ctx, containerCfg, hostCfg, &network.NetworkingConfig{}, nil, "")
	if err != nil {
		return "", fmt.Errorf("failed to create warm container: %w", err)
	}
	if err := p.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		p.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return "", fmt.Errorf("failed to start warm container: %w", err)
	}
	return resp.ID, nil
}

func (p *ContainerPool) Get(language string) (string, error) {
	ch, ok := p.pools[language]
	if !ok {
		return "", fmt.Errorf("no pool for language %s", language)
	}

	select {
	case id := <-ch:
		p.logger.Info("pool hit", "language", language, "container_id", id[:12])
		return id, nil
	default:
		p.logger.Warn("pool miss, creating container on-demand", "language", language)
		return p.createWarmContainer(language)
	}
}

func (p *ContainerPool) Recycle(language, containerID string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := p.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
		if err != nil {
			p.logger.Error("failed to remove user container", "container_id", containerID[:12])
		}
		p.mu.Lock()
		isShutdown := p.shutdown
		p.mu.Unlock()

		if isShutdown {
			return
		}

		newID, err := p.createWarmContainer(language)
		if err != nil {
			p.logger.Error("failed to create replacement container", "language", language, "error", err)
			return
		}

		select {
		case p.pools[language] <- newID:
			p.logger.Info("recycled container", "language", language, "new_container_id", newID[:12])
		default:
			p.client.ContainerRemove(context.Background(), newID, container.RemoveOptions{Force: true})
		}
	}()
}

func (p *ContainerPool) Close() {
	p.mu.Lock()
	p.shutdown = true
	p.mu.Unlock()

	for lang, ch := range p.pools {
		close(ch)
		for id := range ch {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := p.client.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
			if err != nil {
				p.logger.Error("failed to remove pool container during shutdown", "language", lang, "container_id", id[:12], "error", err)
			} else {
				p.logger.Info("removed pool container", "language", lang, "container_id", id[:12])
			}
			cancel()
		}
	}
}

func int64Ptr(i int64) *int64 {
	return &i
}
