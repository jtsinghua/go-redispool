package src

type QueueEmptyError struct {
	msg string
}

func (err QueueEmptyError) Error() string {
	return err.msg
}

//超时错误
type TimeoutError struct {
	msg string
}

func (err TimeoutError) Error() string {
	return err.msg
}
