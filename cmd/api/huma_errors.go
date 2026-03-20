package main

import "github.com/danielgtaylor/huma/v2"

// humaAPIError standardizes Huma-generated errors to match the
// existing Rails/spec response shape used elsewhere in the API.
type humaAPIError struct {
	status  int
	Code    int    `json:"code"`
	Message string `json:"error"`
}

func (e *humaAPIError) Error() string {
	return e.Message
}

func (e *humaAPIError) GetStatus() int {
	return e.status
}

// humaAPIErrorNoCode matches Grape's built-in validation error envelope,
// e.g. {"error":"query is missing"} with an HTTP 400 status.
type humaAPIErrorNoCode struct {
	status  int
	Message string `json:"error"`
}

func (e *humaAPIErrorNoCode) Error() string {
	return e.Message
}

func (e *humaAPIErrorNoCode) GetStatus() int {
	return e.status
}

func newAPIError(status int, msg string) huma.StatusError {
	return &humaAPIError{
		status:  status,
		Code:    status,
		Message: msg,
	}
}

func newAPIErrorNoCode(status int, msg string) huma.StatusError {
	return &humaAPIErrorNoCode{
		status:  status,
		Message: msg,
	}
}

func configureHumaErrors() {
	huma.NewError = func(status int, msg string, errs ...error) huma.StatusError {
		return newAPIError(status, msg)
	}

	huma.NewErrorWithContext = func(_ huma.Context, status int, msg string, errs ...error) huma.StatusError {
		return huma.NewError(status, msg, errs...)
	}
}

