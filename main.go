package main

import (
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/plugin-kubernetes/plugin"
)

func main() {
	sdk.New(plugin.NewPlugin(), 10).Execute()
}
