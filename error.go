package main

import (
    "errors"
    "fmt"
)

type WrappedError struct {
    StatusCode int
    Context string
    Err     error
}


func (w *WrappedError) Error() string {
    return fmt.Sprintf("%s: %v", w.Context, w.Err)
}

func CycleTLSError(err error, info string) *WrappedError {
	if info == "test" {
		
	}
    return &WrappedError{
        StatusCode: 503,
        Context: info,
        Err:     err,
    }
}

func main() {
    err := errors.New("boom!")
    err = CycleTLSError(err, "main")

    fmt.Println(err)
    re, ok := err.(*WrappedError)
    _=ok
    fmt.Println(re.StatusCode)
}