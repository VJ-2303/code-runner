package runner

import (
	"context"
	"fmt"

	pb "github.com/VJ-2303/code-runner/proto/runner"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GRPCRunnerClient struct {
	client pb.RunnerServiceClient
	conn   *grpc.ClientConn
}

func NewGRPCRunnerClient(workerAddr string) (*GRPCRunnerClient, error) {
	conn, err := grpc.NewClient(workerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to worker at %s: %w", workerAddr, err)
	}
	client := pb.NewRunnerServiceClient(conn)

	return &GRPCRunnerClient{
		client: client,
		conn:   conn,
	}, nil
}

func (c *GRPCRunnerClient) Run(ctx context.Context, code, language, stdin string) (*ExecuteResult, error) {
	resp, err := c.client.RunCode(ctx, &pb.RunCodeRequest{
		Code:     code,
		Language: language,
		Stdin:    stdin,
	})
	if err != nil {
		return nil, fmt.Errorf("worker RPC failed: %w", err)
	}

	return &ExecuteResult{
		Output: resp.Output,
		Error:  resp.Error,
	}, nil
}

func (c *GRPCRunnerClient) Close() error {
	return c.conn.Close()
}
