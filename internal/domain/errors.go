package domain

// ErrorCode identifies a category of domain error.
type ErrorCode string

const (
	ErrorNotFound     ErrorCode = "not_found"
	ErrorInvalidInput ErrorCode = "invalid_input"
	ErrorConflict     ErrorCode = "conflict"
)

// DomainError is an adapter-agnostic error payload.
type DomainError struct {
	Code    ErrorCode
	Message string
}

func (e DomainError) Error() string {
	return string(e.Code) + ": " + e.Message
}

func NewNotFound(message string) DomainError {
	return DomainError{Code: ErrorNotFound, Message: message}
}

func NewInvalidInput(message string) DomainError {
	return DomainError{Code: ErrorInvalidInput, Message: message}
}

func NewConflict(message string) DomainError {
	return DomainError{Code: ErrorConflict, Message: message}
}
