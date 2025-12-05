package security

import (
	"sync"
	"sync/atomic"
	"time"
)

// ResourceLimits defines resource limits for a plugin.
type ResourceLimits struct {
	// Memory limit in bytes (advisory - not strictly enforced)
	MemoryLimit int64

	// Maximum execution time per call
	ExecutionTimeout time.Duration

	// Maximum instructions per execution (Lua VM)
	InstructionLimit int64

	// Maximum file operations per second
	FileOpsPerSecond int

	// Maximum network requests per second
	NetworkReqPerSecond int

	// Maximum concurrent goroutines
	MaxGoroutines int

	// Maximum output size in bytes
	MaxOutputSize int64
}

// DefaultResourceLimits returns sensible default limits.
func DefaultResourceLimits() ResourceLimits {
	return ResourceLimits{
		MemoryLimit:         10 * 1024 * 1024, // 10 MB
		ExecutionTimeout:    5 * time.Second,
		InstructionLimit:    10_000_000,
		FileOpsPerSecond:    100,
		NetworkReqPerSecond: 10,
		MaxGoroutines:       10,
		MaxOutputSize:       1 * 1024 * 1024, // 1 MB
	}
}

// StrictResourceLimits returns stricter limits for untrusted plugins.
func StrictResourceLimits() ResourceLimits {
	return ResourceLimits{
		MemoryLimit:         5 * 1024 * 1024, // 5 MB
		ExecutionTimeout:    2 * time.Second,
		InstructionLimit:    1_000_000,
		FileOpsPerSecond:    10,
		NetworkReqPerSecond: 1,
		MaxGoroutines:       2,
		MaxOutputSize:       256 * 1024, // 256 KB
	}
}

// RelaxedResourceLimits returns relaxed limits for trusted plugins.
func RelaxedResourceLimits() ResourceLimits {
	return ResourceLimits{
		MemoryLimit:         50 * 1024 * 1024, // 50 MB
		ExecutionTimeout:    30 * time.Second,
		InstructionLimit:    100_000_000,
		FileOpsPerSecond:    1000,
		NetworkReqPerSecond: 100,
		MaxGoroutines:       50,
		MaxOutputSize:       10 * 1024 * 1024, // 10 MB
	}
}

// ResourceMonitor tracks resource usage and enforces limits.
type ResourceMonitor struct {
	mu sync.RWMutex

	limits ResourceLimits

	// Tracking
	instructionCount int64
	memoryUsage      int64
	goroutineCount   int32
	outputSize       int64

	// Rate limiters
	fileOpsLimiter    *RateLimiter
	networkReqLimiter *RateLimiter

	// State
	exceeded bool
	reason   string
}

// NewResourceMonitor creates a new resource monitor with the given limits.
func NewResourceMonitor(limits ResourceLimits) *ResourceMonitor {
	return &ResourceMonitor{
		limits:            limits,
		fileOpsLimiter:    NewRateLimiter(limits.FileOpsPerSecond),
		networkReqLimiter: NewRateLimiter(limits.NetworkReqPerSecond),
	}
}

// IncrementInstructions increments the instruction counter.
// Returns true if the limit is exceeded.
func (rm *ResourceMonitor) IncrementInstructions(count int64) bool {
	newCount := atomic.AddInt64(&rm.instructionCount, count)
	if rm.limits.InstructionLimit > 0 && newCount > rm.limits.InstructionLimit {
		rm.setExceeded("instruction limit exceeded")
		return true
	}
	return false
}

// InstructionCount returns the current instruction count.
func (rm *ResourceMonitor) InstructionCount() int64 {
	return atomic.LoadInt64(&rm.instructionCount)
}

// ResetInstructionCount resets the instruction counter.
func (rm *ResourceMonitor) ResetInstructionCount() {
	atomic.StoreInt64(&rm.instructionCount, 0)
}

// UpdateMemoryUsage updates the memory usage tracking.
// Returns true if the limit is exceeded.
func (rm *ResourceMonitor) UpdateMemoryUsage(bytes int64) bool {
	atomic.StoreInt64(&rm.memoryUsage, bytes)
	if rm.limits.MemoryLimit > 0 && bytes > rm.limits.MemoryLimit {
		rm.setExceeded("memory limit exceeded")
		return true
	}
	return false
}

// MemoryUsage returns the current memory usage.
func (rm *ResourceMonitor) MemoryUsage() int64 {
	return atomic.LoadInt64(&rm.memoryUsage)
}

// IncrementGoroutines increments the goroutine counter.
// Returns true if the limit is exceeded.
func (rm *ResourceMonitor) IncrementGoroutines() bool {
	newCount := atomic.AddInt32(&rm.goroutineCount, 1)
	if rm.limits.MaxGoroutines > 0 && int(newCount) > rm.limits.MaxGoroutines {
		rm.setExceeded("goroutine limit exceeded")
		return true
	}
	return false
}

// DecrementGoroutines decrements the goroutine counter.
func (rm *ResourceMonitor) DecrementGoroutines() {
	atomic.AddInt32(&rm.goroutineCount, -1)
}

// GoroutineCount returns the current goroutine count.
func (rm *ResourceMonitor) GoroutineCount() int {
	return int(atomic.LoadInt32(&rm.goroutineCount))
}

// AddOutput adds to the output size tracker.
// Returns true if the limit is exceeded.
func (rm *ResourceMonitor) AddOutput(bytes int64) bool {
	newSize := atomic.AddInt64(&rm.outputSize, bytes)
	if rm.limits.MaxOutputSize > 0 && newSize > rm.limits.MaxOutputSize {
		rm.setExceeded("output size limit exceeded")
		return true
	}
	return false
}

// OutputSize returns the current output size.
func (rm *ResourceMonitor) OutputSize() int64 {
	return atomic.LoadInt64(&rm.outputSize)
}

// ResetOutputSize resets the output size counter.
func (rm *ResourceMonitor) ResetOutputSize() {
	atomic.StoreInt64(&rm.outputSize, 0)
}

// TryFileOp attempts to perform a file operation.
// Returns true if allowed, false if rate limited.
func (rm *ResourceMonitor) TryFileOp() bool {
	if !rm.fileOpsLimiter.Allow() {
		rm.setExceeded("file operation rate limit exceeded")
		return false
	}
	return true
}

// TryNetworkRequest attempts to perform a network request.
// Returns true if allowed, false if rate limited.
func (rm *ResourceMonitor) TryNetworkRequest() bool {
	if !rm.networkReqLimiter.Allow() {
		rm.setExceeded("network request rate limit exceeded")
		return false
	}
	return true
}

// ExecutionTimeout returns the execution timeout.
func (rm *ResourceMonitor) ExecutionTimeout() time.Duration {
	return rm.limits.ExecutionTimeout
}

// Limits returns the current limits.
func (rm *ResourceMonitor) Limits() ResourceLimits {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.limits
}

// SetLimits updates the resource limits.
func (rm *ResourceMonitor) SetLimits(limits ResourceLimits) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.limits = limits
	rm.fileOpsLimiter = NewRateLimiter(limits.FileOpsPerSecond)
	rm.networkReqLimiter = NewRateLimiter(limits.NetworkReqPerSecond)
}

// IsExceeded returns true if any limit was exceeded.
func (rm *ResourceMonitor) IsExceeded() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.exceeded
}

// ExceededReason returns the reason for limit exceeded, if any.
func (rm *ResourceMonitor) ExceededReason() string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.reason
}

// setExceeded marks limits as exceeded with a reason.
func (rm *ResourceMonitor) setExceeded(reason string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.exceeded = true
	rm.reason = reason
}

// Reset resets all counters and clears exceeded state.
func (rm *ResourceMonitor) Reset() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	atomic.StoreInt64(&rm.instructionCount, 0)
	atomic.StoreInt64(&rm.memoryUsage, 0)
	atomic.StoreInt32(&rm.goroutineCount, 0)
	atomic.StoreInt64(&rm.outputSize, 0)
	rm.exceeded = false
	rm.reason = ""
}

// RateLimiter implements a simple token bucket rate limiter.
type RateLimiter struct {
	mu sync.Mutex

	rate       int       // operations per second
	tokens     int       // current tokens
	maxTokens  int       // maximum tokens (burst size)
	lastRefill time.Time // last token refill time
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(ratePerSecond int) *RateLimiter {
	if ratePerSecond <= 0 {
		// No limit
		return &RateLimiter{rate: 0, tokens: 1, maxTokens: 1}
	}
	return &RateLimiter{
		rate:       ratePerSecond,
		tokens:     ratePerSecond,
		maxTokens:  ratePerSecond,
		lastRefill: time.Now(),
	}
}

// Allow returns true if an operation is allowed.
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// No limit
	if rl.rate == 0 {
		return true
	}

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)
	tokensToAdd := int(elapsed.Seconds() * float64(rl.rate))
	if tokensToAdd > 0 {
		rl.tokens += tokensToAdd
		if rl.tokens > rl.maxTokens {
			rl.tokens = rl.maxTokens
		}
		rl.lastRefill = now
	}

	// Check if we have tokens
	if rl.tokens <= 0 {
		return false
	}

	rl.tokens--
	return true
}

// Reset resets the rate limiter to full capacity.
func (rl *RateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.tokens = rl.maxTokens
	rl.lastRefill = time.Now()
}

// ResourceUsage represents a snapshot of resource usage.
type ResourceUsage struct {
	InstructionCount int64
	MemoryUsage      int64
	GoroutineCount   int
	OutputSize       int64
	Exceeded         bool
	ExceededReason   string
}

// GetUsage returns a snapshot of current resource usage.
func (rm *ResourceMonitor) GetUsage() ResourceUsage {
	rm.mu.RLock()
	exceeded := rm.exceeded
	reason := rm.reason
	rm.mu.RUnlock()

	return ResourceUsage{
		InstructionCount: rm.InstructionCount(),
		MemoryUsage:      rm.MemoryUsage(),
		GoroutineCount:   rm.GoroutineCount(),
		OutputSize:       rm.OutputSize(),
		Exceeded:         exceeded,
		ExceededReason:   reason,
	}
}
