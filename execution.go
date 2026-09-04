package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/adnanh/webhook/internal/hook"
)

var (
	commandTimeout     = flag.String("command-timeout", "", "default timeout for hook command execution (for example 30s); 0 disables the timeout")
	maxConcurrency     = flag.Int("max-concurrency", 0, "default maximum number of concurrent executions per hook; 0 disables the limit")
	defaultExecTimeout time.Duration

	errHookConcurrencyLimit = errors.New("hook concurrency limit reached")
)

type hookExecutionLimiter struct {
	limit  int
	tokens chan struct{}
}

var hookExecutionLimiters = struct {
	mu     sync.Mutex
	byHook map[string]*hookExecutionLimiter
}{
	byHook: make(map[string]*hookExecutionLimiter),
}

func initExecutionSettings() error {
	value := strings.TrimSpace(*commandTimeout)
	switch value {
	case "", "0":
		defaultExecTimeout = 0
	default:
		timeout, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid command-timeout: %w", err)
		}
		if timeout < 0 {
			return errors.New("command-timeout must be zero or greater")
		}
		defaultExecTimeout = timeout
	}

	if *maxConcurrency < 0 {
		return errors.New("max-concurrency must be zero or greater")
	}

	return nil
}

func handleHook(h *hook.Hook, r *hook.Request) (string, error) {
	release, err := acquireHookExecutionSlot(h)
	if err != nil {
		return "", err
	}

	return executeHook(h, r, true, release)
}

func handleHookAsync(h *hook.Hook, r *hook.Request) error {
	release, err := acquireHookExecutionSlot(h)
	if err != nil {
		return err
	}

	go func() {
		_, _ = executeHook(h, r, false, release)
	}()

	return nil
}

func hookExecutionErrorStatus(err error) (int, string) {
	if errors.Is(err, errHookConcurrencyLimit) {
		return http.StatusServiceUnavailable, "Hook concurrency limit exceeded. Please try again later."
	}

	return http.StatusInternalServerError, "Error occurred while executing the hook's command. Please check your logs for more details."
}

func acquireHookExecutionSlot(h *hook.Hook) (func(), error) {
	limit, err := h.EffectiveMaxConcurrency(*maxConcurrency)
	if err != nil {
		return nil, err
	}
	if limit == 0 {
		return func() {}, nil
	}

	limiter := executionLimiterForHook(h.ID, limit)

	select {
	case limiter.tokens <- struct{}{}:
		return func() {
			<-limiter.tokens
		}, nil
	default:
		return nil, fmt.Errorf("%w (%d running)", errHookConcurrencyLimit, limit)
	}
}

func executionLimiterForHook(id string, limit int) *hookExecutionLimiter {
	hookExecutionLimiters.mu.Lock()
	defer hookExecutionLimiters.mu.Unlock()

	limiter := hookExecutionLimiters.byHook[id]
	if limiter == nil || limiter.limit != limit {
		limiter = &hookExecutionLimiter{
			limit:  limit,
			tokens: make(chan struct{}, limit),
		}
		hookExecutionLimiters.byHook[id] = limiter
	}

	return limiter
}

func hookExecutionContext(h *hook.Hook, r *hook.Request, waitForCommand bool) (context.Context, context.CancelFunc, bool, error) {
	timeout, err := h.EffectiveCommandTimeout(defaultExecTimeout)
	if err != nil {
		return nil, nil, false, err
	}

	ctx := context.Background()
	useContext := timeout > 0

	if waitForCommand && r != nil && r.RawRequest != nil {
		ctx = r.RawRequest.Context()
		useContext = true
	}

	if timeout > 0 {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		return ctx, cancel, true, nil
	}

	return ctx, func() {}, useContext, nil
}
