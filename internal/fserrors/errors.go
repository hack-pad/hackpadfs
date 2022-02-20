package fserrors

type messageError struct {
	err     error
	message string
}

// WithMessage annotates 'err' with the prefix 'message'. Returns nil if 'err' is nil.
func WithMessage(err error, message string) error {
	if err == nil {
		return nil
	}
	return &messageError{
		err:     err,
		message: message,
	}
}

func (m *messageError) Error() string {
	return m.message + ": " + m.err.Error()
}

func (m *messageError) Unwrap() error {
	return m.err
}
