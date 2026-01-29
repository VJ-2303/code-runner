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

	"github.com/VJ-2303/code-runner/internal/data"
	"github.com/VJ-2303/code-runner/internal/mailer"
	"github.com/VJ-2303/code-runner/internal/runner"
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
	redis struct {
		addr     string
		password string
	}
	limiter struct {
		enabled bool
		limit   int
		window  int
	}
	smtp struct {
		host     string
		port     int
		username string
		password string
		sender   string
	}
}

type application struct {
	config config
	logger *slog.Logger
	models data.Models
	runner runner.Runner
	mailer mailer.Mailer
	redis  *redis.Client
}

func main() {
	var cfg config

	godotenv.Load()

	flag.IntVar(&cfg.port, "port", 4000, "API server port")
	flag.StringVar(&cfg.env, "env", "devolopment", "Environment (devolopment|staging|production)")

	flag.StringVar(&cfg.db.dsn, "db-dsn", "postgres://code_runner_user:pa55word@localhost/code_runner?sslmode=disable", "PostgreSQL DSN")
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.DurationVar(&cfg.db.maxIdleTime, "db-max-idle-time", 15*time.Minute, "PostgreSQL max connection idle lifetime")

	flag.StringVar(&cfg.redis.addr, "redis-addr", "localhost:6379", "Redis Address")
	flag.StringVar(&cfg.redis.password, "redis-pass", "pa55word", "Redis password")

	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")
	flag.IntVar(&cfg.limiter.limit, "limiter-limit", 20, "Ratelimiter limit")
	flag.IntVar(&cfg.limiter.window, "limiter-window", 60, "Ratelimiter window size")

	flag.StringVar(&cfg.smtp.host, "smtp-host", "smtp.gmail.com", "SMTP host")
	flag.IntVar(&cfg.smtp.port, "smtp-port", 587, "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-user", "vanaraj1018@gmail.com", "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-pass", os.Getenv("SMTPPASS"), "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", "Code Runner <no-reply@coderunner.net>", "SMTP sender")

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

	redisDB, err := openRedis(cfg)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	defer redisDB.Close()

	logger.Info("redis database connection established")

	app := &application{
		config: cfg,
		logger: logger,
		models: data.NewModels(db),
		runner: runner.NewDockerRunner(),
		mailer: mailer.New(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.sender),
		redis:  redisDB,
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.port),
		Handler:      app.router(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
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

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	err = srv.Shutdown(ctx)
	if err != nil {
		logger.Error("gracefull shutdown failed", "error", err)
		srv.Close()
	}
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

func openRedis(cfg config) (*redis.Client, error) {
	rdb := redis.NewClient(
		&redis.Options{
			Addr:     cfg.redis.addr,
			Password: cfg.redis.password,
		},
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}
	return rdb, nil
}
