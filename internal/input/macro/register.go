package macro

import "unicode"

// Register validation constants.
const (
	// MinLetterRegister is the first valid letter register.
	MinLetterRegister = 'a'
	// MaxLetterRegister is the last valid letter register.
	MaxLetterRegister = 'z'
	// MinDigitRegister is the first valid digit register.
	MinDigitRegister = '0'
	// MaxDigitRegister is the last valid digit register.
	MaxDigitRegister = '9'
)

// IsValidRegister returns true if r is a valid register name.
// Valid registers are lowercase letters (a-z) and digits (0-9).
func IsValidRegister(r rune) bool {
	return IsLetterRegister(r) || IsDigitRegister(r)
}

// IsLetterRegister returns true if r is a letter register (a-z).
func IsLetterRegister(r rune) bool {
	return r >= MinLetterRegister && r <= MaxLetterRegister
}

// IsDigitRegister returns true if r is a digit register (0-9).
func IsDigitRegister(r rune) bool {
	return r >= MinDigitRegister && r <= MaxDigitRegister
}

// NormalizeRegister converts a register to its canonical form.
// Uppercase letters are converted to lowercase.
// Invalid registers return 0.
func NormalizeRegister(r rune) rune {
	// Handle uppercase letters
	if r >= 'A' && r <= 'Z' {
		return unicode.ToLower(r)
	}
	// Validate and return
	if IsValidRegister(r) {
		return r
	}
	return 0
}

// IsAppendRegister returns true if r is an uppercase letter (A-Z).
// In Vim, uppercase letters append to the corresponding lowercase register.
func IsAppendRegister(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

// ToAppendTarget converts an uppercase register to its lowercase target.
// Returns the lowercase letter for A-Z, or 0 for invalid input.
func ToAppendTarget(r rune) rune {
	if IsAppendRegister(r) {
		return unicode.ToLower(r)
	}
	return 0
}

// AllLetterRegisters returns all valid letter registers (a-z).
func AllLetterRegisters() []rune {
	result := make([]rune, 0, 26)
	for r := MinLetterRegister; r <= MaxLetterRegister; r++ {
		result = append(result, r)
	}
	return result
}

// AllDigitRegisters returns all valid digit registers (0-9).
func AllDigitRegisters() []rune {
	result := make([]rune, 0, 10)
	for r := MinDigitRegister; r <= MaxDigitRegister; r++ {
		result = append(result, r)
	}
	return result
}

// AllRegisters returns all valid registers (a-z, 0-9).
func AllRegisters() []rune {
	return append(AllLetterRegisters(), AllDigitRegisters()...)
}

// RegisterInfo provides metadata about a register.
type RegisterInfo struct {
	// Name is the register name (a-z or 0-9).
	Name rune

	// EventCount is the number of events in the register.
	EventCount int

	// IsEmpty is true if the register contains no events.
	IsEmpty bool
}

// GetRegisterInfo returns information about a register's contents.
func GetRegisterInfo(recorder *Recorder, register rune) RegisterInfo {
	if !IsValidRegister(register) {
		return RegisterInfo{Name: register, IsEmpty: true}
	}

	count := recorder.EventCount(register)
	return RegisterInfo{
		Name:       register,
		EventCount: count,
		IsEmpty:    count == 0,
	}
}

// GetAllRegisterInfo returns information about all registers.
func GetAllRegisterInfo(recorder *Recorder) []RegisterInfo {
	registers := AllRegisters()
	result := make([]RegisterInfo, 0, len(registers))

	for _, r := range registers {
		info := GetRegisterInfo(recorder, r)
		// Only include non-empty registers
		if !info.IsEmpty {
			result = append(result, info)
		}
	}

	return result
}
