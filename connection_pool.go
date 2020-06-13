package pool

type ConnClient struct {
	conn net.Conn
	factoryFunc func() (net.Conn, error)
}

func newConnClient(factoryFunc func() (net.Conn, error)) (client *ConnClient, err error) {
	conn, err := factoryFunc()
	if err != nil {
		return nil, err
	}
	return &ConnClient{
		conn: conn,
		factoryFunc: factoryFunc,
	}, nil
}

func (c *ConnClient) Recreate() (err error) {
	conn, err := c.factoryFunc()
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

func (c *ConnClient) Close() {
	c.conn.Close()
}

func (c *ConnClient) Healthy() bool {
	return c.conn != nil
}

type ConnPool struct {
	mu sync.RWMutex
	pool chan RenewableClient
	capacity int
}

func NewConnPool(capacity int, initLen int, connCreateFunc func() (*ConnPool, error)) {
	if initLen < 0 || capacity <= 0 || initLen > capacity {
		return nil, fmt.Errorf("invalid capacity or init length")
	}

	pool := &{
		pool: make(chan RenewableClient, capacity),
		capacity: capacity,
	}

	for i = 0; i < initLen; i++ {
		conn, err := newConnClient()
		if err != nil {
			return nil, fmt.Errorf("create client error, err=%s", err)
		}
		pool.pool <- conn
	}

	return pool, nil
}

func (p *ConnPool) Len() int {
	return len(p.pool)
}

func (p *ConnPool) Capacity() int {
	return p.capacity
}

func (p *ConnPool) Close() error {
	p.mu.Lock()
	connChan := p.pool
	p.pool = nil
	p.mu.Unlock()

	if connPool == nil {
		return
	}

	close(connChan)

	for conn := range connChan {
		conn.Close()
	}
}

func (p *ConnPool) Get() (client RenewableClient, err error) {
	p.mu.Lock()
	client, ok := <-p.pool
	p.mu.Unlock()
	if ! ok {
		return nil, fmt.Errorf("get client error, err=%s", err)
	}
	return client, nil
}

func (p *ConnPool) GetAndRun(
	retryAttempt int,
	executeFunc func(client RenewableClient) error,
	errCallback func(attempNum int, err error)
) (err error) {
	if retryAttempt < 0 {
		return fmt.Errorf("invalid retry attemp times")
	}

	p.mu.Lock()
	client, ok := <-p.pool
	p.mu.Unlock()
	defer func(){
		p.mu.Lock()
		p.pool <- client
		p.mu.Unlock()
	}()

	for i := 0; i < retryAttempt; i++ {
		if !client.Healthy() {
			err = client.Recreate()
			if err != nil {
				continue
			}
		}

		err = executeFunc(client)
		if err != nil {
			errCallback(i, err)
			client.Recreate()
			continue
		}
		return nil
	}

	return err
}