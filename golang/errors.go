package main

import (
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

func parseError(err error) (errormessage errorMessage) {
	var msg string
	var debugger string
	var op string

	httpError := string(err.Error())
	status := lastString(strings.Split(httpError, "StatusCode:"))
	StatusCode, _ := strconv.Atoi(status)
	if StatusCode != 0 {
		msg = fmt.Sprintf("Request error")
		debugger = fmt.Sprintf("%#v\n", err)
		errormessage = errorMessage{StatusCode: StatusCode, debugger: debugger, ErrorMsg: msg}
		return
	}
	if uerr, ok := err.(*url.Error); ok {
		if noerr, ok := uerr.Err.(*net.OpError); ok {
			op = noerr.Op
			if SyscallError, ok := noerr.Err.(*os.SyscallError); ok {
				msg = fmt.Sprintf("Proxy returned a Syscall Error: %s", SyscallError)
				debugger = fmt.Sprintf("%#v\n", SyscallError)
				if noerr.Timeout() {
					errormessage = errorMessage{StatusCode: 408, debugger: debugger, ErrorMsg: msg, Op: op}
				}
				errormessage = errorMessage{StatusCode: 401, debugger: debugger, ErrorMsg: msg, Op: op}
				return

			} else if AddrError, ok := noerr.Err.(*net.AddrError); ok {
				msg = fmt.Sprintf("Proxy returned a Addr Error: %s", AddrError)
				debugger = fmt.Sprintf("%#v\n", AddrError)
				errormessage = errorMessage{StatusCode: 405, debugger: debugger, ErrorMsg: msg, Op: op}
				return

			} else if DNSError, ok := noerr.Err.(*net.DNSError); ok {
				msg = fmt.Sprintf("Proxy returned a DNS Error: %s", DNSError)
				debugger = fmt.Sprintf("%#v\n", DNSError)
				errormessage = errorMessage{StatusCode: 421, debugger: debugger, ErrorMsg: msg, Op: op}
				return
			} else {
				msg = fmt.Sprintf("Proxy returned a Error: %s", noerr)
				debugger = fmt.Sprintf("%#v\n", noerr)
				errormessage = errorMessage{StatusCode: 400, debugger: debugger, ErrorMsg: msg, Op: op}
				return
			}
		}
		if uerr.Timeout() {
			msg = fmt.Sprintf("Proxy returned a Timeout Error: %s", uerr)
			debugger = fmt.Sprintf("%#v\n", uerr)
			errormessage = errorMessage{StatusCode: 408, debugger: debugger, ErrorMsg: msg, Op: op}
			return
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
