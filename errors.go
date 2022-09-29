package ecspresso

type ErrSkipVerify string

func (e ErrSkipVerify) Error() string {
	return string(e)
}

type ErrNotFound string

func (e ErrNotFound) Error() string {
	return string(e)
}

var (
	errNotFound   = ErrNotFound("not found")
	errSkipVerify = ErrSkipVerify("skip verify")
)
