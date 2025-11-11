package httputil

import (
	"errors"
	"net/http"

	"gocloud.dev/gcerrors"
)

func NewError(err error, httpStatusCode int) error {
	if err == nil {
		return nil
	}

	if 600 <= httpStatusCode || httpStatusCode < 100 {
		httpStatusCode = 500
	}

	return &httpStatusCodeError{
		err:            err,
		httpStatusCode: httpStatusCode,
	}
}

type httpStatusCodeError struct {
	err            error
	httpStatusCode int
}

func (e *httpStatusCodeError) Error() string {
	if e.err == nil {
		return ""
	}

	return e.err.Error()
}

func (e *httpStatusCodeError) Unwrap() error {
	return e.err
}

func HTTPStatusCode(err error) int {
	hscerr := &httpStatusCodeError{}
	if errors.As(err, &hscerr) {
		return hscerr.httpStatusCode
	}

	switch gcerrors.Code(err) {
	case gcerrors.NotFound:
		return http.StatusNotFound
	case gcerrors.AlreadyExists:
		return http.StatusConflict
	case gcerrors.InvalidArgument:
		return http.StatusBadRequest
	case gcerrors.FailedPrecondition:
		return http.StatusPreconditionFailed
	case gcerrors.PermissionDenied:
		return http.StatusForbidden
	case gcerrors.ResourceExhausted:
		return http.StatusInsufficientStorage
	}

	return http.StatusInternalServerError
}
