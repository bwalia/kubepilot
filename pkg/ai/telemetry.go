package ai

import (
	"context"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/kubepilot/kubepilot/pkg/telemetry"
)

// chatCompletion issues one request to the LLM, recording a client span and the
// GenAI metrics around it.
//
// Every model call in this package goes through here. Centralising it means
// latency, token usage, and failure rate are captured identically for command
// interpretation, pod troubleshooting, and root cause analysis — and that the
// call sites stay about their prompts rather than their instrumentation.
//
// The task argument names the KubePilot operation ("interpret", "rca",
// "troubleshoot") so one model's cost can be attributed across the features
// that use it.
func (e *Engine) chatCompletion(ctx context.Context, task string, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	tel := telemetry.Default()
	start := time.Now()

	ctx, span := tel.Tracer.Start(ctx, "chat "+req.Model,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gen_ai.system", "ollama"),
			attribute.String("gen_ai.operation.name", "chat"),
			attribute.String("gen_ai.request.model", req.Model),
			attribute.Float64("gen_ai.request.temperature", float64(req.Temperature)),
			attribute.String("kubepilot.ai.task", task),
		),
	)
	defer span.End()

	resp, err := e.client.CreateChatCompletion(ctx, req)

	attrs := []attribute.KeyValue{
		attribute.String("gen_ai.system", "ollama"),
		attribute.String("gen_ai.operation.name", "chat"),
		attribute.String("gen_ai.request.model", req.Model),
		attribute.String("kubepilot.ai.task", task),
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "LLM call failed")
		tel.Metrics.AIErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
		tel.Metrics.AIDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(append(attrs, attribute.String("error.type", "error"))...))
		return resp, err
	}

	// Ollama reports usage on the OpenAI-compatible endpoint, but not every
	// backend does; zero counts are simply not recorded.
	if resp.Usage.PromptTokens > 0 {
		tel.Metrics.AITokens.Add(ctx, int64(resp.Usage.PromptTokens),
			metric.WithAttributes(append(attrs, attribute.String("gen_ai.token.type", "input"))...))
		span.SetAttributes(attribute.Int("gen_ai.usage.input_tokens", resp.Usage.PromptTokens))
	}
	if resp.Usage.CompletionTokens > 0 {
		tel.Metrics.AITokens.Add(ctx, int64(resp.Usage.CompletionTokens),
			metric.WithAttributes(append(attrs, attribute.String("gen_ai.token.type", "output"))...))
		span.SetAttributes(attribute.Int("gen_ai.usage.output_tokens", resp.Usage.CompletionTokens))
	}
	if len(resp.Choices) > 0 {
		span.SetAttributes(attribute.String("gen_ai.response.finish_reason",
			string(resp.Choices[0].FinishReason)))
	}

	tel.Metrics.AIDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
	return resp, nil
}
