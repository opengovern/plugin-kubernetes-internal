package main

import (
	"context"
	"github.com/kaytu-io/kaytu/cmd"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/opengovern/plugin-kubernetes-internal/plugin"
)

func main() {
	ctx := cmd.AppendSignalHandling(context.Background())
	sdk.New(plugin.NewPlugin(), 5).Execute(ctx)
}
