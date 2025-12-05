package security

import (
	"testing"
	"time"
)

func TestDefaultResourceLimits(t *testing.T) {
	limits := DefaultResourceLimits()

	if limits.MemoryLimit != 10*1024*1024 {
		t.Errorf("MemoryLimit = %d, want %d", limits.MemoryLimit, 10*1024*1024)
	}
	if limits.ExecutionTimeout != 5*time.Second {
		t.Errorf("ExecutionTimeout = %v, want %v", limits.ExecutionTimeout, 5*time.Second)
	}
	if limits.InstructionLimit != 10_000_000 {
		t.Errorf("InstructionLimit = %d, want %d", limits.InstructionLimit, 10_000_000)
	}
	if limits.FileOpsPerSecond != 100 {
		t.Errorf("FileOpsPerSecond = %d, want %d", limits.FileOpsPerSecond, 100)
	}
	if limits.NetworkReqPerSecond != 10 {
		t.Errorf("NetworkReqPerSecond = %d, want %d", limits.NetworkReqPerSecond, 10)
	}
	if limits.MaxGoroutines != 10 {
		t.Errorf("MaxGoroutines = %d, want %d", limits.MaxGoroutines, 10)
	}
	if limits.MaxOutputSize != 1*1024*1024 {
		t.Errorf("MaxOutputSize = %d, want %d", limits.MaxOutputSize, 1*1024*1024)
	}
}

func TestStrictResourceLimits(t *testing.T) {
	limits := StrictResourceLimits()

	if limits.MemoryLimit != 5*1024*1024 {
		t.Errorf("MemoryLimit = %d, want %d", limits.MemoryLimit, 5*1024*1024)
	}
	if limits.ExecutionTimeout != 2*time.Second {
		t.Errorf("ExecutionTimeout = %v, want %v", limits.ExecutionTimeout, 2*time.Second)
	}
	if limits.InstructionLimit != 1_000_000 {
		t.Errorf("InstructionLimit = %d, want %d", limits.InstructionLimit, 1_000_000)
	}
}

func TestRelaxedResourceLimits(t *testing.T) {
	limits := RelaxedResourceLimits()

	if limits.MemoryLimit != 50*1024*1024 {
		t.Errorf("MemoryLimit = %d, want %d", limits.MemoryLimit, 50*1024*1024)
	}
	if limits.ExecutionTimeout != 30*time.Second {
		t.Errorf("ExecutionTimeout = %v, want %v", limits.ExecutionTimeout, 30*time.Second)
	}
	if limits.InstructionLimit != 100_000_000 {
		t.Errorf("InstructionLimit = %d, want %d", limits.InstructionLimit, 100_000_000)
	}
}

func TestNewResourceMonitor(t *testing.T) {
	limits := DefaultResourceLimits()
	rm := NewResourceMonitor(limits)

	if rm == nil {
		t.Fatal("NewResourceMonitor returned nil")
	}
	if rm.fileOpsLimiter == nil {
		t.Error("fileOpsLimiter is nil")
	}
	if rm.networkReqLimiter == nil {
		t.Error("networkReqLimiter is nil")
	}
}

func TestResourceMonitorInstructions(t *testing.T) {
	limits := ResourceLimits{
		InstructionLimit: 1000,
	}
	rm := NewResourceMonitor(limits)

	// Below limit
	exceeded := rm.IncrementInstructions(500)
	if exceeded {
		t.Error("IncrementInstructions(500) should not exceed limit of 1000")
	}
	if rm.InstructionCount() != 500 {
		t.Errorf("InstructionCount() = %d, want 500", rm.InstructionCount())
	}

	// At limit
	exceeded = rm.IncrementInstructions(500)
	if exceeded {
		t.Error("IncrementInstructions(500) should not exceed limit at 1000")
	}

	// Exceed limit
	exceeded = rm.IncrementInstructions(1)
	if !exceeded {
		t.Error("IncrementInstructions(1) should exceed limit of 1000")
	}
	if !rm.IsExceeded() {
		t.Error("IsExceeded() should be true")
	}
	if rm.ExceededReason() != "instruction limit exceeded" {
		t.Errorf("ExceededReason() = %q, want %q", rm.ExceededReason(), "instruction limit exceeded")
	}

	// Reset
	rm.ResetInstructionCount()
	if rm.InstructionCount() != 0 {
		t.Errorf("InstructionCount() after reset = %d, want 0", rm.InstructionCount())
	}
}

func TestResourceMonitorMemory(t *testing.T) {
	limits := ResourceLimits{
		MemoryLimit: 1000,
	}
	rm := NewResourceMonitor(limits)

	// Below limit
	exceeded := rm.UpdateMemoryUsage(500)
	if exceeded {
		t.Error("UpdateMemoryUsage(500) should not exceed limit of 1000")
	}
	if rm.MemoryUsage() != 500 {
		t.Errorf("MemoryUsage() = %d, want 500", rm.MemoryUsage())
	}

	// Exceed limit
	exceeded = rm.UpdateMemoryUsage(1500)
	if !exceeded {
		t.Error("UpdateMemoryUsage(1500) should exceed limit of 1000")
	}
	if !rm.IsExceeded() {
		t.Error("IsExceeded() should be true")
	}
}

func TestResourceMonitorGoroutines(t *testing.T) {
	limits := ResourceLimits{
		MaxGoroutines: 3,
	}
	rm := NewResourceMonitor(limits)

	// Below limit
	for i := 0; i < 3; i++ {
		exceeded := rm.IncrementGoroutines()
		if exceeded {
			t.Errorf("IncrementGoroutines() iteration %d should not exceed limit", i)
		}
	}
	if rm.GoroutineCount() != 3 {
		t.Errorf("GoroutineCount() = %d, want 3", rm.GoroutineCount())
	}

	// Exceed limit
	exceeded := rm.IncrementGoroutines()
	if !exceeded {
		t.Error("IncrementGoroutines() should exceed limit of 3")
	}

	// Decrement
	rm.DecrementGoroutines()
	if rm.GoroutineCount() != 3 {
		t.Errorf("GoroutineCount() after decrement = %d, want 3", rm.GoroutineCount())
	}
}

func TestResourceMonitorOutput(t *testing.T) {
	limits := ResourceLimits{
		MaxOutputSize: 1000,
	}
	rm := NewResourceMonitor(limits)

	// Below limit
	exceeded := rm.AddOutput(500)
	if exceeded {
		t.Error("AddOutput(500) should not exceed limit of 1000")
	}
	if rm.OutputSize() != 500 {
		t.Errorf("OutputSize() = %d, want 500", rm.OutputSize())
	}

	// Exceed limit
	exceeded = rm.AddOutput(600)
	if !exceeded {
		t.Error("AddOutput(600) should exceed limit of 1000")
	}

	// Reset
	rm.ResetOutputSize()
	if rm.OutputSize() != 0 {
		t.Errorf("OutputSize() after reset = %d, want 0", rm.OutputSize())
	}
}

func TestResourceMonitorFileOps(t *testing.T) {
	limits := ResourceLimits{
		FileOpsPerSecond: 2,
	}
	rm := NewResourceMonitor(limits)

	// Should allow first few operations
	if !rm.TryFileOp() {
		t.Error("TryFileOp() #1 should succeed")
	}
	if !rm.TryFileOp() {
		t.Error("TryFileOp() #2 should succeed")
	}

	// Should be rate limited
	if rm.TryFileOp() {
		t.Error("TryFileOp() #3 should be rate limited")
	}
}

func TestResourceMonitorNetworkReq(t *testing.T) {
	limits := ResourceLimits{
		NetworkReqPerSecond: 2,
	}
	rm := NewResourceMonitor(limits)

	// Should allow first few operations
	if !rm.TryNetworkRequest() {
		t.Error("TryNetworkRequest() #1 should succeed")
	}
	if !rm.TryNetworkRequest() {
		t.Error("TryNetworkRequest() #2 should succeed")
	}

	// Should be rate limited
	if rm.TryNetworkRequest() {
		t.Error("TryNetworkRequest() #3 should be rate limited")
	}
}

func TestResourceMonitorExecutionTimeout(t *testing.T) {
	limits := ResourceLimits{
		ExecutionTimeout: 5 * time.Second,
	}
	rm := NewResourceMonitor(limits)

	if rm.ExecutionTimeout() != 5*time.Second {
		t.Errorf("ExecutionTimeout() = %v, want %v", rm.ExecutionTimeout(), 5*time.Second)
	}
}

func TestResourceMonitorLimits(t *testing.T) {
	limits := DefaultResourceLimits()
	rm := NewResourceMonitor(limits)

	got := rm.Limits()
	if got.MemoryLimit != limits.MemoryLimit {
		t.Errorf("Limits().MemoryLimit = %d, want %d", got.MemoryLimit, limits.MemoryLimit)
	}
}

func TestResourceMonitorSetLimits(t *testing.T) {
	limits := DefaultResourceLimits()
	rm := NewResourceMonitor(limits)

	newLimits := StrictResourceLimits()
	rm.SetLimits(newLimits)

	got := rm.Limits()
	if got.MemoryLimit != newLimits.MemoryLimit {
		t.Errorf("Limits().MemoryLimit = %d, want %d", got.MemoryLimit, newLimits.MemoryLimit)
	}
}

func TestResourceMonitorReset(t *testing.T) {
	limits := DefaultResourceLimits()
	rm := NewResourceMonitor(limits)

	// Set some values
	rm.IncrementInstructions(1000)
	rm.UpdateMemoryUsage(500)
	rm.IncrementGoroutines()
	rm.AddOutput(100)

	// Verify they're set
	if rm.InstructionCount() != 1000 {
		t.Errorf("InstructionCount() = %d before reset", rm.InstructionCount())
	}

	// Reset
	rm.Reset()

	// Verify everything is cleared
	if rm.InstructionCount() != 0 {
		t.Errorf("InstructionCount() = %d after reset, want 0", rm.InstructionCount())
	}
	if rm.MemoryUsage() != 0 {
		t.Errorf("MemoryUsage() = %d after reset, want 0", rm.MemoryUsage())
	}
	if rm.GoroutineCount() != 0 {
		t.Errorf("GoroutineCount() = %d after reset, want 0", rm.GoroutineCount())
	}
	if rm.OutputSize() != 0 {
		t.Errorf("OutputSize() = %d after reset, want 0", rm.OutputSize())
	}
	if rm.IsExceeded() {
		t.Error("IsExceeded() should be false after reset")
	}
}

func TestResourceMonitorGetUsage(t *testing.T) {
	limits := DefaultResourceLimits()
	rm := NewResourceMonitor(limits)

	rm.IncrementInstructions(1000)
	rm.UpdateMemoryUsage(500)
	rm.IncrementGoroutines()
	rm.AddOutput(100)

	usage := rm.GetUsage()

	if usage.InstructionCount != 1000 {
		t.Errorf("usage.InstructionCount = %d, want 1000", usage.InstructionCount)
	}
	if usage.MemoryUsage != 500 {
		t.Errorf("usage.MemoryUsage = %d, want 500", usage.MemoryUsage)
	}
	if usage.GoroutineCount != 1 {
		t.Errorf("usage.GoroutineCount = %d, want 1", usage.GoroutineCount)
	}
	if usage.OutputSize != 100 {
		t.Errorf("usage.OutputSize = %d, want 100", usage.OutputSize)
	}
	if usage.Exceeded {
		t.Error("usage.Exceeded should be false")
	}
}

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter(3) // 3 per second

	// Should allow burst
	if !rl.Allow() {
		t.Error("Allow() #1 should succeed")
	}
	if !rl.Allow() {
		t.Error("Allow() #2 should succeed")
	}
	if !rl.Allow() {
		t.Error("Allow() #3 should succeed")
	}

	// Should be rate limited
	if rl.Allow() {
		t.Error("Allow() #4 should be rate limited")
	}
}

func TestRateLimiterNoLimit(t *testing.T) {
	rl := NewRateLimiter(0) // No limit

	// Should always allow
	for i := 0; i < 100; i++ {
		if !rl.Allow() {
			t.Errorf("Allow() iteration %d should succeed with no limit", i)
		}
	}
}

func TestRateLimiterReset(t *testing.T) {
	rl := NewRateLimiter(2)

	// Exhaust tokens
	rl.Allow()
	rl.Allow()
	if rl.Allow() {
		t.Error("Should be rate limited before reset")
	}

	// Reset
	rl.Reset()

	// Should have tokens again
	if !rl.Allow() {
		t.Error("Allow() should succeed after reset")
	}
}

func TestRateLimiterRefill(t *testing.T) {
	rl := NewRateLimiter(100) // High rate to test refill

	// Exhaust tokens
	for i := 0; i < 100; i++ {
		rl.Allow()
	}

	// Wait for refill
	time.Sleep(50 * time.Millisecond)

	// Should have some tokens refilled
	if !rl.Allow() {
		t.Error("Allow() should succeed after refill period")
	}
}

func TestResourceMonitorNoLimits(t *testing.T) {
	// Test with zero limits (no enforcement)
	limits := ResourceLimits{
		InstructionLimit:    0,
		MemoryLimit:         0,
		MaxGoroutines:       0,
		MaxOutputSize:       0,
		FileOpsPerSecond:    0,
		NetworkReqPerSecond: 0,
	}
	rm := NewResourceMonitor(limits)

	// None should exceed
	if rm.IncrementInstructions(1000000000) {
		t.Error("IncrementInstructions should not exceed with 0 limit")
	}
	if rm.UpdateMemoryUsage(1000000000) {
		t.Error("UpdateMemoryUsage should not exceed with 0 limit")
	}
	if rm.IncrementGoroutines() {
		t.Error("IncrementGoroutines should not exceed with 0 limit")
	}
	if rm.AddOutput(1000000000) {
		t.Error("AddOutput should not exceed with 0 limit")
	}
}
