package services

// part of TCC (try-confirm-cancel)
// this is for cross service transaction like processes
// ex job service creates job -> email service fails create job, rollback job service
type ServiceResponse interface {
	Commit()
	Cancel()
	Data() any
}

type CommitFunc func()
type CancelFunc func()

type GenericServiceResponse struct {
	CommitOp CommitFunc
	CancelOp CancelFunc
	OpData   any
}

func (r *GenericServiceResponse) Commit() {
	r.CommitOp()
}

func (r *GenericServiceResponse) Cancel() {
	r.CancelOp()
}

func (r *GenericServiceResponse) Data() any {
	return r.OpData
}

func NewServiceResponse(commit CommitFunc, cancel CancelFunc, data any) {}
