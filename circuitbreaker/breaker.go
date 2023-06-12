/*
The MIT License (MIT)

# Copyright 2015 Sony Corporation

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

// Package circuitbreaker implements the [Circuit Breaker] pattern in Go
//
// [Circuit Breaker]: https://learn.microsoft.com/en-us/azure/architecture/patterns/circuit-breaker
package circuitbreaker

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// State represent the state of the CircuitBreaker
type State int

const (
	// requests allowed
	StateClosed State = iota

	// only a certain number of requests allowed to probe success
	StateHalfOpen

	// requests fail immediately
	StateOpen
)

var (
	// ErrTooManyRequests is returned when the CircuitBreaker state is half open
	// and the current request count is over the maxRequests
	ErrTooManyRequests = errors.New("too many requests")

	// ErrOpenState is returned when the CircuitBreaker state is open
	ErrOpenState = errors.New("circuit breaker is open")
)

// String implements the stringer interface
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateHalfOpen:
		return "half-open"
	case StateOpen:
		return "open"
	default:
		return fmt.Sprintf("unknown state: %d", s)
	}
}

// Counts holds the number of requests and their successes/failures.
// CircuitBreaker clears the internal Counts either on change of state or at
// the closed-state intervals
type Counts struct {
	CurrRequests         uint32
	TotalSuccesses       uint32
	TotalFailures        uint32
	ConsecutiveSuccesses uint32
	ConsecutiveFailures  uint32
}

type Config struct {
	// MaxRequests is the maximum number of requests allowed to pass through
	// when the CircuitBreaker is half-open. If it is set to zero (i.e. no value
	// is set), only 1 request is allowed as the default
	MaxRequests uint32

	// Interval is the interval in the closed state after which the Counts will
	// be reset to zero
	Interval time.Duration

	// Timeout is the period of the open state after which the state of the
	// CircuitBreaker becomes half-open. If Timeout is 0, the timout value of
	// CircuitBreaker is set to 60 seconds as a default
	Timeout time.Duration

	// ShouldTrip is called with Counts whenever a request fails in the closed
	// state. If ShouldTrip returns true, CircuitBreaker is set to the open
	// state. If ShouldTrip is nil, a default callback is used which checks
	// that number of consecutive failures is not more than 5.
	ShouldTrip func(counts Counts) bool

	// OnStateChange is called whenever the state of CircuitBreaker changes
	OnStateChange func(from State, to State)

	// IsSuccessful is called with the error that's returned from a request. If
	// it returns true, the error is counted as a success. Otherwise, the error
	// is counted as a failure. If IsSuccessful is used, a default callback is
	// used which returns false for all non-nil errors
	IsSuccessful func(err error) bool
}

// CircuitBreaker is a state machine  that prevents making requests that are
// likely to fail
type CircuitBreaker struct {
	maxRequests   uint32
	interval      time.Duration
	timeout       time.Duration
	shouldTrip    func(counts Counts) bool
	onStateChange func(from State, to State)
	isSuccessful  func(err error) bool

	mutex      sync.Mutex
	state      State
	generation uint64
	counts     Counts
	expiry     time.Time
}

func (cfg *Config) setDefaults() {
	if cfg.MaxRequests == 0 {
		cfg.MaxRequests = 1
	}

	if cfg.Interval <= 0 {
		cfg.Interval = time.Duration(0) * time.Second
	}

	if cfg.Timeout <= 0 {
		cfg.Timeout = time.Duration(60) * time.Second
	}

	if cfg.ShouldTrip == nil {
		cfg.ShouldTrip = func(counts Counts) bool {
			return counts.ConsecutiveFailures > 5
		}
	}

	if cfg.IsSuccessful == nil {
		cfg.IsSuccessful = func(err error) bool {
			return err == nil
		}
	}
}

// NewCircuitBreaker returns a new instance of CircuitBreaker with the given configuration
func NewCircuitBreaker(cfg Config) *CircuitBreaker {
	cfg.setDefaults()

	cb := &CircuitBreaker{
		onStateChange: cfg.OnStateChange,
		maxRequests:   cfg.MaxRequests,
		interval:      cfg.Interval,
		timeout:       cfg.Timeout,
		shouldTrip:    cfg.ShouldTrip,
		isSuccessful:  cfg.IsSuccessful,
	}
	cb.toNewGeneration(time.Now())
	return cb
}

// State returns the current state of the CircuitBreaker
func (cb *CircuitBreaker) State() State {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	state, _ := cb.currentState(now)
	return state

}

// Counts returns the internal counters
func (cb *CircuitBreaker) Counts() Counts {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	return cb.counts
}

func (cb *CircuitBreaker) beforeRequest() (uint64, error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)

	if state == StateOpen {
		return generation, ErrOpenState
	} else if state == StateHalfOpen && cb.counts.CurrRequests >= cb.maxRequests {
		return generation, ErrTooManyRequests
	}

	cb.counts.CurrRequests++
	return generation, nil
}

// Do runs the given request if the CircuitBreaker accepts it. Do returns an
// error instantly if the CircuitBreaker is opened. Otherwise, Do returns the
// result of the request. If a panic occurs in the request callback, the
// CircuitBreaker handles it as an error and causes the same panic again.
func (cb *CircuitBreaker) Do(req func() (interface{}, error)) (interface{}, error) {
	generation, err := cb.beforeRequest()
	if err != nil {
		return nil, err
	}

	defer func() {
		e := recover()
		if e != nil {
			cb.afterRequest(generation, false)
			panic(e)
		}
	}()

	result, err := req()
	cb.afterRequest(generation, cb.isSuccessful(err))
	return result, err
}

func (cb *CircuitBreaker) toNewGeneration(now time.Time) {
	cb.generation++
	// clear counts
	cb.counts = Counts{}

	var zero time.Time
	switch cb.state {
	case StateClosed:
		if cb.interval == 0 {
			cb.expiry = zero
		} else {
			cb.expiry = now.Add(cb.interval)
		}
	case StateOpen:
		cb.expiry = now.Add(cb.timeout)
	case StateHalfOpen:
		cb.expiry = zero
	}
}

func (cb *CircuitBreaker) currentState(now time.Time) (State, uint64) {
	switch cb.state {
	case StateClosed:
		if !cb.expiry.IsZero() && cb.expiry.Before(now) {
			cb.toNewGeneration(now)
		}
	case StateOpen:
		if cb.expiry.Before(now) {
			cb.setState(StateHalfOpen, now)
		}
	}
	return cb.state, cb.generation
}

func (cb *CircuitBreaker) setState(newState State, now time.Time) {
	if cb.state == newState {
		return
	}

	prev := cb.state
	cb.state = newState

	cb.toNewGeneration(now)

	if cb.onStateChange != nil {
		cb.onStateChange(prev, newState)
	}
}

func (cb *CircuitBreaker) afterRequest(before uint64, success bool) {
	// if state is Open, this function should not be called
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)
	if generation != before {
		return
	}

	if success { // on success
		cb.counts.TotalSuccesses++
		cb.counts.ConsecutiveSuccesses++
		cb.counts.ConsecutiveFailures = 0
		if cb.counts.ConsecutiveSuccesses >= cb.maxRequests {
			cb.setState(StateClosed, now) // no-op if state is already Closed
		}
	} else { // on failure
		switch state {
		case StateClosed:
			cb.counts.TotalFailures++
			cb.counts.ConsecutiveFailures++
			cb.counts.ConsecutiveSuccesses = 0
			if cb.shouldTrip(cb.counts) {
				cb.setState(StateOpen, now)
			}
		case StateHalfOpen:
			// if a faiilure
			cb.setState(StateOpen, now)
		}
	}
}
