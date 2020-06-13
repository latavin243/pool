package pool

type RenewableClient interface {
	Healthy() bool
	Recreate() error
	Close()
}

type Pool interface {
	Get() (RenewableClient, error)
	FillClients(newClientNumber int) error

	Len() int
	Capacity() int
	Close() error

	GetAndRun(
		retryAttempt int,
		executeFunc func(client RenewableClient) error,
		errCallback func(attempNum int, err error),
	) (err error)
}
