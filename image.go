package sindri

import (
	"context"
	"io"
	"os"
)

type Opener interface {
	Open() (io.ReadCloser, error)
	Close() error
}

type FileOpener string

func (o FileOpener) Open() (io.ReadCloser, error) {
	return os.Open(string(o))
}

func (o FileOpener) Close() error {
	return os.Remove(string(o))
}

type ImageBuilder interface {
	BuildImage(context.Context, string, string) (Opener, error)
}
