package cycletls

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type errorMessage struct {
	StatusCode int
	debugger   string
	ErrorMsg   string
	Op         string
}

func lastString(ss []string) string {
	return ss[len(ss)-1]
}

// func createErrorString(err: string) (msg, debugger string) {
func createErrorString(err error) (msg, debugger string) {
	msg = fmt.Sprintf("Request returned a Syscall Error: %s", err)
	debugger = fmt.Sprintf("%#v\n", err)
	return
}

func createErrorMessage(StatusCode int, err error, op string) errorMessage {
	msg := fmt.Sprintf("Request returned a Syscall Error: %s", err)
	debugger := fmt.Sprintf("%#v\n", err)
	return errorMessage{StatusCode: StatusCode, debugger: debugger, ErrorMsg: msg, Op: op}
}

func parseError(err error) (errormessage errorMessage) {
	var op string

	// Check for context.DeadlineExceeded (client timeout)
	if err == context.DeadlineExceeded {
		return createErrorMessage(408, err, "timeout")
	}

	httpError := string(err.Error())
	//todo - clean this up

	// Check for TLS certificate errors (should return 495)
	if strings.Contains(httpError, "uTlsConn.Handshake() error") ||
		strings.Contains(httpError, "tls: failed to verify certificate") ||
		strings.Contains(httpError, "x509: certificate") ||
		strings.Contains(httpError, "certificate verify failed") ||
		strings.Contains(httpError, "certificate has expired") ||
		strings.Contains(httpError, "certificate signed by unknown authority") {
		return createErrorMessage(495, err, "certificate")
	}

	// Check for connection refused errors (should return 502)
	if strings.Contains(httpError, "connection refused") ||
		strings.Contains(httpError, "connect: connection refused") ||
		strings.Contains(httpError, "dial tcp") && strings.Contains(httpError, "connect: connection refused") {
		return createErrorMessage(502, err, "connection")
	}

	// Check for common timeout error messages
	if strings.Contains(httpError, "context deadline exceeded") ||
		strings.Contains(httpError, "Client.Timeout exceeded") ||
		strings.Contains(httpError, "timeout") {
		return createErrorMessage(408, err, "timeout")
	}

	status := lastString(strings.Split(httpError, "StatusCode:"))
	StatusCode, _ := strconv.Atoi(status)
	if StatusCode != 0 {
		msg, debugger := createErrorString(err)
		return errorMessage{StatusCode: StatusCode, debugger: debugger, ErrorMsg: msg}
	}
	if uerr, ok := err.(*url.Error); ok {
		if noerr, ok := uerr.Err.(*net.OpError); ok {
			op = noerr.Op
			if SyscallError, ok := noerr.Err.(*os.SyscallError); ok {
				if noerr.Timeout() {
					return createErrorMessage(408, SyscallError, op)
				}
				return createErrorMessage(401, SyscallError, op)
			} else if AddrError, ok := noerr.Err.(*net.AddrError); ok {
				return createErrorMessage(405, AddrError, op)
			} else if DNSError, ok := noerr.Err.(*net.DNSError); ok {
				return createErrorMessage(421, DNSError, op)
			} else {
				return createErrorMessage(421, noerr, op)
			}
		}
		if uerr.Timeout() {
			return createErrorMessage(408, uerr, op)
		}
	}
	return
}

type errExtensionNotExist struct {
	Context string
}

func (w *errExtensionNotExist) Error() string {
	return fmt.Sprintf("Extension {{ %s }} is not Supported by CycleTLS please raise an issue", w.Context)
}

func raiseExtensionError(info string) *errExtensionNotExist {
	return &errExtensionNotExist{
		Context: info,
	}
}
