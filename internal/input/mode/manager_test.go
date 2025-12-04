package mode

import (
	"testing"
)

func TestManagerRegister(t *testing.T) {
	m := NewManager()

	normal := NewNormalMode()
	m.Register(normal)

	if got := m.Get(ModeNormal); got == nil {
		t.Error("Get(normal) should return registered mode")
	}

	modes := m.Modes()
	found := false
	for _, name := range modes {
		if name == ModeNormal {
			found = true
			break
		}
	}
	if !found {
		t.Error("Modes() should include registered mode")
	}
}

func TestManagerUnregister(t *testing.T) {
	m := NewManager()
	m.Register(NewNormalMode())
	m.Register(NewInsertMode())

	if err := m.Unregister(ModeInsert); err != nil {
		t.Errorf("Unregister() error = %v", err)
	}

	if m.Get(ModeInsert) != nil {
		t.Error("Get() should return nil after Unregister")
	}
}

func TestManagerUnregisterCurrent(t *testing.T) {
	m := NewManager()
	m.Register(NewNormalMode())
	_ = m.SetInitialMode(ModeNormal)

	err := m.Unregister(ModeNormal)
	if err == nil {
		t.Error("Unregister current mode should fail")
	}
}

func TestManagerSetInitialMode(t *testing.T) {
	m := NewManager()
	m.Register(NewNormalMode())

	if err := m.SetInitialMode(ModeNormal); err != nil {
		t.Errorf("SetInitialMode() error = %v", err)
	}

	if m.CurrentName() != ModeNormal {
		t.Errorf("CurrentName() = %q, want %q", m.CurrentName(), ModeNormal)
	}
}

func TestManagerSetInitialModeUnknown(t *testing.T) {
	m := NewManager()

	err := m.SetInitialMode("unknown")
	if err == nil {
		t.Error("SetInitialMode with unknown mode should fail")
	}
}

func TestManagerSwitch(t *testing.T) {
	m := NewManager()
	m.Register(NewNormalMode())
	m.Register(NewInsertMode())
	_ = m.SetInitialMode(ModeNormal)

	if err := m.Switch(ModeInsert); err != nil {
		t.Errorf("Switch() error = %v", err)
	}

	if m.CurrentName() != ModeInsert {
		t.Errorf("CurrentName() after Switch = %q, want %q", m.CurrentName(), ModeInsert)
	}

	prev := m.Previous()
	if prev == nil || prev.Name() != ModeNormal {
		t.Errorf("Previous() = %v, want normal mode", prev)
	}
}

func TestManagerSwitchUnknown(t *testing.T) {
	m := NewManager()
	m.Register(NewNormalMode())
	_ = m.SetInitialMode(ModeNormal)

	err := m.Switch("unknown")
	if err == nil {
		t.Error("Switch to unknown mode should fail")
	}
}

func TestManagerPushPop(t *testing.T) {
	m := NewManager()
	m.Register(NewNormalMode())
	m.Register(NewOperatorPendingMode())
	_ = m.SetInitialMode(ModeNormal)

	if m.StackDepth() != 0 {
		t.Errorf("initial StackDepth() = %d, want 0", m.StackDepth())
	}

	// Push
	if err := m.Push(ModeOperatorPending); err != nil {
		t.Errorf("Push() error = %v", err)
	}

	if m.CurrentName() != ModeOperatorPending {
		t.Errorf("CurrentName() after Push = %q, want %q", m.CurrentName(), ModeOperatorPending)
	}
	if m.StackDepth() != 1 {
		t.Errorf("StackDepth() after Push = %d, want 1", m.StackDepth())
	}

	// Pop
	if err := m.Pop(); err != nil {
		t.Errorf("Pop() error = %v", err)
	}

	if m.CurrentName() != ModeNormal {
		t.Errorf("CurrentName() after Pop = %q, want %q", m.CurrentName(), ModeNormal)
	}
	if m.StackDepth() != 0 {
		t.Errorf("StackDepth() after Pop = %d, want 0", m.StackDepth())
	}
}

func TestManagerPopEmpty(t *testing.T) {
	m := NewManager()
	m.Register(NewNormalMode())
	_ = m.SetInitialMode(ModeNormal)

	err := m.Pop()
	if err == nil {
		t.Error("Pop on empty stack should fail")
	}
}

func TestManagerIsMode(t *testing.T) {
	m := NewManager()
	m.Register(NewNormalMode())
	m.Register(NewInsertMode())
	_ = m.SetInitialMode(ModeNormal)

	if !m.IsMode(ModeNormal) {
		t.Error("IsMode(normal) should be true")
	}
	if m.IsMode(ModeInsert) {
		t.Error("IsMode(insert) should be false")
	}
}

func TestManagerIsAnyMode(t *testing.T) {
	m := NewManager()
	m.Register(NewNormalMode())
	m.Register(NewInsertMode())
	m.Register(NewVisualMode())
	_ = m.SetInitialMode(ModeVisual)

	if !m.IsAnyMode(ModeVisual, ModeVisualLine, ModeVisualBlock) {
		t.Error("IsAnyMode with visual should be true")
	}
	if m.IsAnyMode(ModeNormal, ModeInsert) {
		t.Error("IsAnyMode without visual should be false")
	}
}

func TestManagerOnChange(t *testing.T) {
	m := NewManager()
	m.Register(NewNormalMode())
	m.Register(NewInsertMode())
	_ = m.SetInitialMode(ModeNormal)

	var fromName, toName string
	callCount := 0

	unregister := m.OnChange(func(from, to Mode) {
		callCount++
		if from != nil {
			fromName = from.Name()
		}
		toName = to.Name()
	})

	_ = m.Switch(ModeInsert)

	if callCount != 1 {
		t.Errorf("callback called %d times, want 1", callCount)
	}
	if fromName != ModeNormal {
		t.Errorf("from = %q, want %q", fromName, ModeNormal)
	}
	if toName != ModeInsert {
		t.Errorf("to = %q, want %q", toName, ModeInsert)
	}

	// Unregister
	unregister()

	_ = m.Switch(ModeNormal)

	// Callback should not be called again
	if callCount != 1 {
		t.Errorf("callback called %d times after unregister, want 1", callCount)
	}
}

func TestManagerCurrentWithNoMode(t *testing.T) {
	m := NewManager()

	if m.Current() != nil {
		t.Error("Current() should be nil when no mode set")
	}
	if m.CurrentName() != "" {
		t.Errorf("CurrentName() = %q, want empty", m.CurrentName())
	}
	if m.Previous() != nil {
		t.Error("Previous() should be nil")
	}
}

func TestManagerSwitchWithContext(t *testing.T) {
	m := NewManager()
	m.Register(NewNormalMode())
	m.Register(NewOperatorPendingMode())
	_ = m.SetInitialMode(ModeNormal)

	ctx := NewContext()
	ctx.Extra["operator"] = "d"
	ctx.Extra["count"] = 2

	if err := m.SwitchWithContext(ModeOperatorPending, ctx); err != nil {
		t.Errorf("SwitchWithContext() error = %v", err)
	}

	// The operator-pending mode should have received the context
	opMode := m.Current().(*OperatorPendingMode)
	if opMode.Operator() != "d" {
		t.Errorf("Operator() = %q, want %q", opMode.Operator(), "d")
	}
}

func TestManagerMultiplePushPop(t *testing.T) {
	m := NewManager()
	m.Register(NewNormalMode())
	m.Register(NewInsertMode())
	m.Register(NewCommandMode())
	_ = m.SetInitialMode(ModeNormal)

	// Push multiple modes
	_ = m.Push(ModeInsert)
	_ = m.Push(ModeCommand)

	if m.StackDepth() != 2 {
		t.Errorf("StackDepth() = %d, want 2", m.StackDepth())
	}
	if m.CurrentName() != ModeCommand {
		t.Errorf("CurrentName() = %q, want %q", m.CurrentName(), ModeCommand)
	}

	// Pop back
	_ = m.Pop()
	if m.CurrentName() != ModeInsert {
		t.Errorf("CurrentName() after 1st pop = %q, want %q", m.CurrentName(), ModeInsert)
	}

	_ = m.Pop()
	if m.CurrentName() != ModeNormal {
		t.Errorf("CurrentName() after 2nd pop = %q, want %q", m.CurrentName(), ModeNormal)
	}
}
