package circuitbreaker

type TwoStepCircuitBreaker struct {
	cb *CircuitBreaker
}

func NewTwoStepCircuitBreaker(cfg Config) *TwoStepCircuitBreaker {
	return &TwoStepCircuitBreaker{
		cb: NewCircuitBreaker(cfg),
	}
}

func (tscb *TwoStepCircuitBreaker) State() State {
	return tscb.cb.State()
}

func (tscb *TwoStepCircuitBreaker) Counts() Counts {
	return tscb.cb.Counts()
}

func (tscb *TwoStepCircuitBreaker) Allow() (done func(success bool), err error) {
	generation, err := tscb.cb.beforeRequest()
	if err != nil {
		return nil, err
	}

	return func(success bool) {
		tscb.cb.afterRequest(generation, success)
	}, nil
}
