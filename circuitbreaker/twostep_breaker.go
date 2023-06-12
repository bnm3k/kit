package circuitbreaker

// TwoStepCircuitBreaker provides the same functionality as a CircuitBreaker but
// does not wrap a request, instead it checks whether a request can proceed and
// excepts the caller to report the outcome in a separate step using a callback
type TwoStepCircuitBreaker struct {
	cb *CircuitBreaker
}

// NewTwoStepCircuitBreaker returns a new instance of a TwoStepCircuitBreaker
// with the given configuration.
func NewTwoStepCircuitBreaker(cfg Config) *TwoStepCircuitBreaker {
	return &TwoStepCircuitBreaker{
		cb: NewCircuitBreaker(cfg),
	}
}

// State returns the current state
func (tscb *TwoStepCircuitBreaker) State() State {
	return tscb.cb.State()
}

// Counts returns the internal counters
func (tscb *TwoStepCircuitBreaker) Counts() Counts {
	return tscb.cb.Counts()
}

// Allow checks if a new request can proceed. It returns a callback that should
// be used to register the success or failure in a separate step. If the circuit
// breaker doesn't allow requests, it returns an error.
func (tscb *TwoStepCircuitBreaker) Allow() (done func(success bool), err error) {
	generation, err := tscb.cb.beforeRequest()
	if err != nil {
		return nil, err
	}

	return func(success bool) {
		tscb.cb.afterRequest(generation, success)
	}, nil
}
