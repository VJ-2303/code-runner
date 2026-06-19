package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/VJ-2303/code-runner/internal/broker"
	"github.com/VJ-2303/code-runner/internal/data"
	"github.com/VJ-2303/code-runner/internal/mailer"
	"github.com/VJ-2303/code-runner/internal/ratelimiter"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

type config struct {
	port int
	env  string
	db   struct {
		dsn          string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime  time.Duration
	}
	smtp struct {
		host     string
		port     int
		username string
		password string
		sender   string
	}
	redisURL string
	rabbitmq struct {
		url string
	}
}

type application struct {
	config   config
	logger   *slog.Logger
	models   data.Models
	mailer   mailer.Mailer
	limiter  *ratelimiter.RateLimiter
	rabbitmq *broker.RabbitMQ
}

func main() {
	var cfg config

	godotenv.Load()

	flag.IntVar(&cfg.port, "port", 4000, "API server port")
	flag.StringVar(&cfg.env, "env", os.Getenv("ENV"), "Environment (devolopment|staging|production)")

	flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("DB_DSN"), "PostgreSQL DSN")
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.DurationVar(&cfg.db.maxIdleTime, "db-max-idle-time", 15*time.Minute, "PostgreSQL max connection idle lifetime")

	flag.StringVar(&cfg.smtp.host, "smtp-host", os.Getenv("SMTP_HOST"), "SMTP host")
	flag.IntVar(&cfg.smtp.port, "smtp-port", 587, "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-user", os.Getenv("SMTP_USER"), "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-pass", os.Getenv("SMTPPASS"), "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", os.Getenv("SMTP_SENDER"), "SMTP sender")

	flag.StringVar(&cfg.redisURL, "redis-url", os.Getenv("REDIS_URL"), "Redis server URL")

	flag.StringVar(&cfg.rabbitmq.url, "rabbitmq-url", os.Getenv("RABBITMQ_URL"), "RabbitMQ connection URL")

	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	logger.Info("connecting to database")

	db, err := openDB(cfg)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	defer db.Close()

	logger.Info("postgres database connection pool established")

	redisClient := redis.NewClient(&redis.Options{
		Addr: cfg.redisURL,
	})

	// Verify Redis connection.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	logger.Info("redis connection established")

	logger.Info("connecting to rabbitmq")

	rmq, err := broker.NewRabbitMQ(cfg.rabbitmq.url)
	if err != nil {
		logger.Error("failed to connect to rabbitmq", "error", err)
		os.Exit(1)
	}
	defer rmq.Close()

	app := &application{
		config:   cfg,
		logger:   logger,
		models:   data.NewModels(db),
		mailer:   mailer.New(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.sender),
		limiter:  ratelimiter.New(redisClient),
		rabbitmq: rmq,
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.port),
		Handler:      app.router(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 20 * time.Second,
	}

	go func() {
		logger.Info("starting server", "addr", cfg.port, "env", cfg.env)
		err := srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			logger.Error(err.Error())
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	s := <-quit

	logger.Info("shutting down server", "signal", s.String())

	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	err = srv.Shutdown(ctx)
	if err != nil {
		logger.Error("gracefull shutdown failed", "error", err)
		srv.Close()
	}

	redisClient.Close()
	logger.Info("server stopped")
}

func openDB(cfg config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.db.dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(cfg.db.maxOpenConns)
	db.SetMaxIdleConns(cfg.db.maxIdleConns)
	db.SetConnMaxIdleTime(cfg.db.maxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}
	return db, nil
}
