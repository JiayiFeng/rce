package process

import "context"

type Process interface {
	ID() string
	Kill(ctx context.Context) error
	CWD() string
}
