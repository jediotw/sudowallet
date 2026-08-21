package errors

import (
	"net/http"
)

// DEFINE CUSTOM ERROR USING STRUCT
type AppError struct {
	StatusCode int
	Code       string
	Message    string
}

// i need to make it work w/ go normal standard error handling
// and this is an nbuilt inteface by golang
// type error interface {
// 	Error() string
// }

// implement the interface
func (e *AppError) Error() string {
	return e.Message
}

// i dont want to manually construct this everywhere so
//let make it a constructor and constructor prevents repetitive creation

func NewAppError(StatusCode int, Code string, Message string) *AppError {
	return &AppError{
		StatusCode: StatusCode,
		Code:       Code,
		Message:    Message,
	}
}

// common sentianl errors in an application
// define the error standard that mostly use
var (
	ErrInternalServer   = NewAppError(http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Something went wrong on the server, please try again later.")
	ErrBadRequest       = NewAppError(http.StatusBadRequest, "BAD_REQUEST", "Bad request, please check the request payload.")
	ErrNotFound         = NewAppError(http.StatusNotFound, "NOT_FOUND", "Resource not found.")
	ErrUnauthorized     = NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized, please check the credentials.")
	ErrWalletNotFound   = NewAppError(http.StatusNotFound, "WALLET_NOT_FOUND", "Wallet not found.")
	ErrConcurrentUpdate = NewAppError(http.StatusConflict, "CONCURRENT_UPDATE", "Concurrent update detected, please try again.")
)
