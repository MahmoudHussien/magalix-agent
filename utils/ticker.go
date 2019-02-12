package utils

import (
	"sync"
	"time"
)

type Ticker struct {
	name string

	interval time.Duration
	fn       func()

	mutex *sync.Mutex

	waitChannels []chan struct{}
}

func NewTicker(name string, interval time.Duration, fn func()) *Ticker {
	return &Ticker{
		name:     name,
		interval: interval,
		fn:       fn,

		mutex: &sync.Mutex{},
	}
}

func (ticker *Ticker) nextTick() <-chan time.Time {
	interval := ticker.interval
	if time.Hour%interval == 0 {
		now := time.Now()
		// TODO: sub seconds
		nanos := time.Second*time.Duration(now.Second()) + time.Minute*time.Duration(now.Minute())
		next := interval - nanos%interval
		stderr.Infof(nil, "{%s ticker} next tick after %v", ticker.name, next)
		return time.After(next)
	}
	stderr.Infof(nil, "{%s ticker} next tick after interval %v", ticker.name, interval)
	return time.After(interval)
}

func (ticker *Ticker) unlockWaiting() {
	ticker.mutex.Lock()
	defer ticker.mutex.Unlock()
	for _, waitChan := range ticker.waitChannels {
		waitChan <- struct{}{}
		close(waitChan)
	}
	ticker.waitChannels = make([]chan struct{}, 0)
}

func (ticker *Ticker) tick() {
	ticker.fn()
	// unlock routines waiting for ticks
	ticker.unlockWaiting()
}

// Start start ticker.
// If immediate is true, the ticker tick immediately and blocks for this tick.
// If async is true, each tick firing will run in a different goroutine.
// Else, the tick in the same goroutine as the ticker itself.
// If block is true, Start will block the ticker forever.
// Else, the ticker will run in a different goroutine.
//
// Note: when using async flag:
// 1. if it is true, you may need to apply needed synchronization between ticks.
// Also note that, ticks waiters may got a newer tick unlocking them.
//
// 2. if it is false, the ticker applies the needed synchronizations. In that
// case the ticker don't tick the next one unless the old tick finishes.
// So you may got inconsistent ticks intervals if a tick takes time larger than
// the tick interval to finish. So please consider timeouts if consistent ticks
// are needed.
func (ticker *Ticker) Start(immediate, async, block bool) {
	tickerFn := func() {
		tick := ticker.nextTick()
		for {
			<-tick

			if async {
				go ticker.tick()
			} else {
				ticker.tick()
			}

			tick = ticker.nextTick()
		}
	}

	if immediate {
		// block for first tick
		ticker.fn()
	}
	if block {
		tickerFn()
	} else {
		go tickerFn()
	}
}

// WaitForNextTick returns a signal channel that gets unblocked after the next tick
// Example usage:
//  <- ticker.WaitForNextTick()
func (ticker *Ticker) WaitForNextTick() chan struct{} {
	ticker.mutex.Lock()
	defer ticker.mutex.Unlock()
	waitChan := make(chan struct{})
	ticker.waitChannels = append(ticker.waitChannels, waitChan)
	return waitChan
}
