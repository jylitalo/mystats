package main

import (
	"context"
	"log"
	"os"

	"github.com/jylitalo/mystats/cmd"
	"github.com/jylitalo/mystats/pkg/telemetry"
)

func main() {
	ctx := context.Background()
	telemetryName := os.Getenv("MYSTATS_TELEMETRY")
	if telemetryName == "" {
		telemetryName = "mystats"
	}
	ctx, otel, err := telemetry.Setup(ctx, telemetryName)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = otel.Shutdown(ctx) }()
	tracer := telemetry.GetTracer(ctx)
	ctx, span := tracer.Start(ctx, "start execution")
	defer span.End()
	if err := cmd.Execute(ctx); err != nil {
		log.Fatal(err)
	}
}
