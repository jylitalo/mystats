package main

import (
	"context"
	"log"

	"github.com/jylitalo/mystats/cmd"
	"github.com/jylitalo/mystats/pkg/telemetry"
)

func main() {
	ctx := context.Background()
	ctx, otel, err := telemetry.SetupConsole(ctx, "github.com/jylitalo/mystats")
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
