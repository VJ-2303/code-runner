package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/VJ-2303/code-runner/internal/broker"
	"github.com/VJ-2303/code-runner/internal/data"
	"github.com/VJ-2303/code-runner/internal/runner"
)

type Processor struct {
	logger *slog.Logger
	runner *runner.DockerRunner
	broker *broker.RabbitMQ
	models data.Models
}

func (p *Processor) Start(ctx context.Context) error {
	msgs, err := p.broker.Consume()
	if err != nil {
		return err
	}
	p.logger.Info("worker started, waiting for jobs...")

	for msg := range msgs {
		var payload broker.ExecutionPayload

		err := json.Unmarshal(msg.Body, &payload)
		if err != nil {
			p.logger.Error("failed to unmarshal message", "error", err)

			msg.Nack(false, false)
			continue
		}
		p.logger.Info("received job", "job_id", payload.JobID, "language", payload.Language)

		err = p.ProcessJob(payload)
		if err != nil {
			p.logger.Warn("system error occurred, requeuing job", "job_id", payload.JobID, "error", err)
			msg.Nack(false, true)
			continue
		}

		msg.Ack(false)
	}
	return nil
}

func (p *Processor) ProcessJob(payload broker.ExecutionPayload) error {
	job, err := p.models.Jobs.Get(payload.JobID)
	if err != nil {
		if err.Error() == "record not found" {
			p.logger.Error("job not found in db, discarding", "job_id", payload.JobID)
			return nil
		}
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := p.runner.Run(ctx, payload.Code, payload.Language, payload.Stdin)
	if err != nil {
		p.logger.Error("execution failed", "job_id", payload.JobID, "error", err)
		job.Status = "FAILED"
		errMsg := err.Error()
		job.Error = &errMsg
	} else {
		job.Status = "COMPLETED"
		job.Output = &result.Output
		if result.Error != "" {
			job.Error = &result.Error
		}
	}
	err = p.models.Jobs.Update(job)
	if err != nil {
		p.logger.Error("failed to update job in database", "job_id", payload.JobID, "error", err)
		return err
	} else {
		p.logger.Info("job completed and saved", "job_id", payload.JobID, "status", job.Status)
	}
	return nil
}
