package main

import (
	"context"
	"database/sql"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/VJ-2303/code-runner/internal/broker"
	"github.com/VJ-2303/code-runner/internal/data"
	"github.com/VJ-2303/code-runner/internal/runner"

	_ "github.com/lib/pq"
)

func main() {
	var (
		poolSize    int
		dbDsn       string
		rabbitmqURL string
	)
	flag.IntVar(&poolSize, "pool-size", 3, "Container per language")
	flag.StringVar(&dbDsn, "db-dsn", "postgres://code_runner_user:strongpassword@localhost:5432/code_runner_db?sslmode=disable", "PostgreSQL DSN")
	flag.StringVar(&rabbitmqURL, "rabbitmq-url", "amqp://guest:guest@localhost:5672/", "RabbitMQ connection URL")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	logger.Info("DB_DSN", "db_dsn", dbDsn)

	logger.Info("connecting to database")
	db, err := openDB(dbDsn)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	rmq, err := broker.NewRabbitMQ(rabbitmqURL)
	if err != nil {
		logger.Error("failed to connect to rabbitmq", "error", err)
		os.Exit(1)
	}
	defer rmq.Close()

	logger.Info("initializing docker runner")
	dockerRunner, err := runner.NewDockerRunner(logger, poolSize)
	if err != nil {
		logger.Error("failed to create docker runner", "error", err)
		os.Exit(1)
	}
	defer dockerRunner.Close()

	processor := &Processor{
		logger: logger,
		runner: dockerRunner,
		broker: rmq,
		models: data.NewModels(db),
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		err := processor.Start(ctx)
		if err != nil {
			logger.Error("processor stopped with error", "error", err)
			cancel()
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	s := <-quit
	logger.Info("shutting down worker", "signal", s.String())

	cancel()

	logger.Info("worker stopped successfully")
}

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxIdleTime(15 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}
	return db, nil
}
