package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/VJ-2303/code-runner/internal/runner"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type workerServer struct {
	runner *runner.DockerRunner
	logger *slog.Logger
}

func (s *workerServer) RunCode(ctx context.Context, req *pb.RunCodeRequest) (*pb.RunCodeResponse, error) {
	s.logger.Info("received RunCode request",
		"language", req.Language,
		"code_length", len(req.Code),
		"has_stdin", req.Stdin != "",
	)
	result, err := s.runner.Run(ctx, req.Code, req.Language, req.Stdin)
	if err != nil {
		s.logger.Error("RunCode failed", "error", err)
		return nil, status.Errorf(codes.Internal, "execution failed: %v", err)
	}
	return &pb.RunCodeResponse{
		Output: result.Output,
		Error:  result.Error,
	}, nil
}

func main() {
	var (
		port     int
		poolSize int
	)
	flag.IntVar(&port, "port", 50051, "gRPC server port")
	flag.IntVar(&poolSize, "pool-size", 3, "Container per language")

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	dockerRunner, err := runner.NewDockerRunner(logger, poolSize)
	if err != nil {
		logger.Error("Failed to create docker runner", "error", err)
		os.Exit(1)
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		logger.Error("failed to listen", "port", port, "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()

	pb.RegisterRunnerServiceServer(grpcServer, &workerServer{
		runner: dockerRunner,
		logger: logger,
	})

	go func() {
		logger.Info("worker gRPC server starting", "port", port)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("grpc server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	s := <-quit
	logger.Info("shutting down worker", "signal", s.String())

	grpcServer.GracefulStop()
	dockerRunner.Close()

	logger.Info("worker stopped")
}
