package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/frantjc/sindri/command"
	xerrors "github.com/frantjc/x/errors"
	xos "github.com/frantjc/x/os"
	_ "gocloud.dev/blob/fileblob"
	_ "gocloud.dev/blob/memblob"
	_ "gocloud.dev/blob/s3blob"
)

func main() {
	var (
		ctx, stop = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		cmd       = command.SetCommon(command.NewSindri(), SemVer())
	)

	err := xerrors.Ignore(cmd.ExecuteContext(ctx), context.Canceled)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}

	stop()
	xos.ExitFromError(err)
}
