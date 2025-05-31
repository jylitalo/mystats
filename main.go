package main

import (
	"context"
	"log"
	"os"

	"github.com/jylitalo/mystats/cmd"
	"github.com/jylitalo/mystats/pkg/data"
	"github.com/jylitalo/mystats/pkg/telemetry"
)

func main() {
	ctx := context.Background()
	telemetryName := data.Coalesce(os.Getenv("MYSTATS_TELEMETRY"), "mystats")
	ctx, otel, err := telemetry.Setup(ctx, telemetryName)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = otel.Shutdown(ctx) }()
	ctx, span := telemetry.NewSpan(ctx, "start execution")
	defer span.End()
	if err := cmd.Execute(ctx); err != nil {
		_ = telemetry.Error(span, err)
		span.End()
		log.Fatal(err) //nolint:gocritic
	}
}
