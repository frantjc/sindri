package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/frantjc/sindri/backend/bucket"
	_ "github.com/frantjc/sindri/backend/registry"
	"github.com/frantjc/sindri/command"
	xerrors "github.com/frantjc/x/errors"
	xos "github.com/frantjc/x/os"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	err := xerrors.Ignore(command.NewSindri(SemVer()).ExecuteContext(ctx), context.Canceled)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}

	stop()
	xos.ExitFromError(err)
}
