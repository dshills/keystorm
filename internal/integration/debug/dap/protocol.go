package dap

import (
	"encoding/json"
)

// ProtocolMessage is the base for all DAP messages.
type ProtocolMessage struct {
	Seq  int    `json:"seq"`
	Type string `json:"type"` // "request", "response", "event"
}

// Request represents a DAP request.
type Request struct {
	ProtocolMessage
	Command   string          `json:"command"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// Response represents a DAP response.
type Response struct {
	ProtocolMessage
	RequestSeq int             `json:"request_seq"`
	Success    bool            `json:"success"`
	Command    string          `json:"command"`
	Message    string          `json:"message,omitempty"`
	Body       json.RawMessage `json:"body,omitempty"`
}

// Event represents a DAP event.
type Event struct {
	ProtocolMessage
	Event string          `json:"event"`
	Body  json.RawMessage `json:"body,omitempty"`
}

// ErrorResponse represents a failed response with error details.
type ErrorResponse struct {
	Response
	Body struct {
		Error *ErrorMessage `json:"error,omitempty"`
	} `json:"body,omitempty"`
}

// ErrorMessage contains error details.
type ErrorMessage struct {
	ID        int               `json:"id"`
	Format    string            `json:"format"`
	Variables map[string]string `json:"variables,omitempty"`
}

// Capabilities describes what features the debug adapter supports.
type Capabilities struct {
	SupportsConfigurationDoneRequest      bool `json:"supportsConfigurationDoneRequest,omitempty"`
	SupportsFunctionBreakpoints           bool `json:"supportsFunctionBreakpoints,omitempty"`
	SupportsConditionalBreakpoints        bool `json:"supportsConditionalBreakpoints,omitempty"`
	SupportsHitConditionalBreakpoints     bool `json:"supportsHitConditionalBreakpoints,omitempty"`
	SupportsEvaluateForHovers             bool `json:"supportsEvaluateForHovers,omitempty"`
	SupportsStepBack                      bool `json:"supportsStepBack,omitempty"`
	SupportsSetVariable                   bool `json:"supportsSetVariable,omitempty"`
	SupportsRestartFrame                  bool `json:"supportsRestartFrame,omitempty"`
	SupportsGotoTargetsRequest            bool `json:"supportsGotoTargetsRequest,omitempty"`
	SupportsStepInTargetsRequest          bool `json:"supportsStepInTargetsRequest,omitempty"`
	SupportsCompletionsRequest            bool `json:"supportsCompletionsRequest,omitempty"`
	SupportsModulesRequest                bool `json:"supportsModulesRequest,omitempty"`
	SupportsRestartRequest                bool `json:"supportsRestartRequest,omitempty"`
	SupportsExceptionOptions              bool `json:"supportsExceptionOptions,omitempty"`
	SupportsValueFormattingOptions        bool `json:"supportsValueFormattingOptions,omitempty"`
	SupportsExceptionInfoRequest          bool `json:"supportsExceptionInfoRequest,omitempty"`
	SupportTerminateDebuggee              bool `json:"supportTerminateDebuggee,omitempty"`
	SupportsDelayedStackTraceLoading      bool `json:"supportsDelayedStackTraceLoading,omitempty"`
	SupportsLoadedSourcesRequest          bool `json:"supportsLoadedSourcesRequest,omitempty"`
	SupportsLogPoints                     bool `json:"supportsLogPoints,omitempty"`
	SupportsTerminateThreadsRequest       bool `json:"supportsTerminateThreadsRequest,omitempty"`
	SupportsSetExpression                 bool `json:"supportsSetExpression,omitempty"`
	SupportsTerminateRequest              bool `json:"supportsTerminateRequest,omitempty"`
	SupportsDataBreakpoints               bool `json:"supportsDataBreakpoints,omitempty"`
	SupportsReadMemoryRequest             bool `json:"supportsReadMemoryRequest,omitempty"`
	SupportsDisassembleRequest            bool `json:"supportsDisassembleRequest,omitempty"`
	SupportsCancelRequest                 bool `json:"supportsCancelRequest,omitempty"`
	SupportsBreakpointLocationsRequest    bool `json:"supportsBreakpointLocationsRequest,omitempty"`
	SupportsClipboardContext              bool `json:"supportsClipboardContext,omitempty"`
	SupportsSteppingGranularity           bool `json:"supportsSteppingGranularity,omitempty"`
	SupportsInstructionBreakpoints        bool `json:"supportsInstructionBreakpoints,omitempty"`
	SupportsExceptionFilterOptions        bool `json:"supportsExceptionFilterOptions,omitempty"`
	SupportsSingleThreadExecutionRequests bool `json:"supportsSingleThreadExecutionRequests,omitempty"`
}

// InitializeRequestArguments are the arguments for the initialize request.
type InitializeRequestArguments struct {
	ClientID                     string `json:"clientID,omitempty"`
	ClientName                   string `json:"clientName,omitempty"`
	AdapterID                    string `json:"adapterID"`
	Locale                       string `json:"locale,omitempty"`
	LinesStartAt1                bool   `json:"linesStartAt1,omitempty"`
	ColumnsStartAt1              bool   `json:"columnsStartAt1,omitempty"`
	PathFormat                   string `json:"pathFormat,omitempty"`
	SupportsVariableType         bool   `json:"supportsVariableType,omitempty"`
	SupportsVariablePaging       bool   `json:"supportsVariablePaging,omitempty"`
	SupportsRunInTerminalRequest bool   `json:"supportsRunInTerminalRequest,omitempty"`
	SupportsMemoryReferences     bool   `json:"supportsMemoryReferences,omitempty"`
	SupportsProgressReporting    bool   `json:"supportsProgressReporting,omitempty"`
	SupportsInvalidatedEvent     bool   `json:"supportsInvalidatedEvent,omitempty"`
	SupportsMemoryEvent          bool   `json:"supportsMemoryEvent,omitempty"`
}

// LaunchRequestArguments are the arguments for the launch request.
type LaunchRequestArguments struct {
	NoDebug bool `json:"noDebug,omitempty"`
	// Adapter-specific properties are added by embedding
}

// AttachRequestArguments are the arguments for the attach request.
type AttachRequestArguments struct {
	// Adapter-specific properties
}

// SetBreakpointsArguments are the arguments for setBreakpoints.
type SetBreakpointsArguments struct {
	Source         Source             `json:"source"`
	Breakpoints    []SourceBreakpoint `json:"breakpoints,omitempty"`
	Lines          []int              `json:"lines,omitempty"`
	SourceModified bool               `json:"sourceModified,omitempty"`
}

// SetBreakpointsResponseBody is the response body for setBreakpoints.
type SetBreakpointsResponseBody struct {
	Breakpoints []Breakpoint `json:"breakpoints"`
}

// SetFunctionBreakpointsArguments are the arguments for setFunctionBreakpoints.
type SetFunctionBreakpointsArguments struct {
	Breakpoints []FunctionBreakpoint `json:"breakpoints"`
}

// SetExceptionBreakpointsArguments are the arguments for setExceptionBreakpoints.
type SetExceptionBreakpointsArguments struct {
	Filters          []string                 `json:"filters"`
	FilterOptions    []ExceptionFilterOptions `json:"filterOptions,omitempty"`
	ExceptionOptions []ExceptionOptions       `json:"exceptionOptions,omitempty"`
}

// ContinueArguments are the arguments for continue.
type ContinueArguments struct {
	ThreadID     int  `json:"threadId"`
	SingleThread bool `json:"singleThread,omitempty"`
}

// ContinueResponseBody is the response body for continue.
type ContinueResponseBody struct {
	AllThreadsContinued bool `json:"allThreadsContinued,omitempty"`
}

// NextArguments are the arguments for next (step over).
type NextArguments struct {
	ThreadID     int    `json:"threadId"`
	SingleThread bool   `json:"singleThread,omitempty"`
	Granularity  string `json:"granularity,omitempty"` // "statement", "line", "instruction"
}

// StepInArguments are the arguments for stepIn.
type StepInArguments struct {
	ThreadID     int    `json:"threadId"`
	SingleThread bool   `json:"singleThread,omitempty"`
	TargetID     int    `json:"targetId,omitempty"`
	Granularity  string `json:"granularity,omitempty"`
}

// StepOutArguments are the arguments for stepOut.
type StepOutArguments struct {
	ThreadID     int    `json:"threadId"`
	SingleThread bool   `json:"singleThread,omitempty"`
	Granularity  string `json:"granularity,omitempty"`
}

// PauseArguments are the arguments for pause.
type PauseArguments struct {
	ThreadID int `json:"threadId"`
}

// StackTraceArguments are the arguments for stackTrace.
type StackTraceArguments struct {
	ThreadID   int `json:"threadId"`
	StartFrame int `json:"startFrame,omitempty"`
	Levels     int `json:"levels,omitempty"`
}

// StackTraceResponseBody is the response body for stackTrace.
type StackTraceResponseBody struct {
	StackFrames []StackFrame `json:"stackFrames"`
	TotalFrames int          `json:"totalFrames,omitempty"`
}

// ScopesArguments are the arguments for scopes.
type ScopesArguments struct {
	FrameID int `json:"frameId"`
}

// ScopesResponseBody is the response body for scopes.
type ScopesResponseBody struct {
	Scopes []Scope `json:"scopes"`
}

// VariablesArguments are the arguments for variables.
type VariablesArguments struct {
	VariablesReference int    `json:"variablesReference"`
	Filter             string `json:"filter,omitempty"` // "indexed", "named"
	Start              int    `json:"start,omitempty"`
	Count              int    `json:"count,omitempty"`
}

// VariablesResponseBody is the response body for variables.
type VariablesResponseBody struct {
	Variables []Variable `json:"variables"`
}

// EvaluateArguments are the arguments for evaluate.
type EvaluateArguments struct {
	Expression string `json:"expression"`
	FrameID    int    `json:"frameId,omitempty"`
	Context    string `json:"context,omitempty"` // "watch", "repl", "hover", "clipboard"
}

// EvaluateResponseBody is the response body for evaluate.
type EvaluateResponseBody struct {
	Result             string `json:"result"`
	Type               string `json:"type,omitempty"`
	VariablesReference int    `json:"variablesReference"`
	NamedVariables     int    `json:"namedVariables,omitempty"`
	IndexedVariables   int    `json:"indexedVariables,omitempty"`
	MemoryReference    string `json:"memoryReference,omitempty"`
}

// ThreadsResponseBody is the response body for threads.
type ThreadsResponseBody struct {
	Threads []Thread `json:"threads"`
}

// Source represents a source file.
type Source struct {
	Name             string      `json:"name,omitempty"`
	Path             string      `json:"path,omitempty"`
	SourceReference  int         `json:"sourceReference,omitempty"`
	PresentationHint string      `json:"presentationHint,omitempty"`
	Origin           string      `json:"origin,omitempty"`
	Sources          []Source    `json:"sources,omitempty"`
	AdapterData      interface{} `json:"adapterData,omitempty"`
	Checksums        []Checksum  `json:"checksums,omitempty"`
}

// Checksum represents a checksum for source verification.
type Checksum struct {
	Algorithm string `json:"algorithm"` // "MD5", "SHA1", "SHA256", "timestamp"
	Checksum  string `json:"checksum"`
}

// SourceBreakpoint represents a breakpoint in source.
type SourceBreakpoint struct {
	Line         int    `json:"line"`
	Column       int    `json:"column,omitempty"`
	Condition    string `json:"condition,omitempty"`
	HitCondition string `json:"hitCondition,omitempty"`
	LogMessage   string `json:"logMessage,omitempty"`
}

// FunctionBreakpoint represents a function breakpoint.
type FunctionBreakpoint struct {
	Name         string `json:"name"`
	Condition    string `json:"condition,omitempty"`
	HitCondition string `json:"hitCondition,omitempty"`
}

// Breakpoint represents a verified breakpoint.
type Breakpoint struct {
	ID        int     `json:"id,omitempty"`
	Verified  bool    `json:"verified"`
	Message   string  `json:"message,omitempty"`
	Source    *Source `json:"source,omitempty"`
	Line      int     `json:"line,omitempty"`
	Column    int     `json:"column,omitempty"`
	EndLine   int     `json:"endLine,omitempty"`
	EndColumn int     `json:"endColumn,omitempty"`
	Offset    int     `json:"offset,omitempty"`
}

// ExceptionFilterOptions represents exception filter options.
type ExceptionFilterOptions struct {
	FilterID  string `json:"filterId"`
	Condition string `json:"condition,omitempty"`
}

// ExceptionOptions represents exception breakpoint options.
type ExceptionOptions struct {
	Path      []ExceptionPathSegment `json:"path,omitempty"`
	BreakMode string                 `json:"breakMode"` // "never", "always", "unhandled", "userUnhandled"
}

// ExceptionPathSegment represents a path segment in exception options.
type ExceptionPathSegment struct {
	Negate bool     `json:"negate,omitempty"`
	Names  []string `json:"names"`
}

// Thread represents a thread.
type Thread struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// StackFrame represents a stack frame.
type StackFrame struct {
	ID                          int         `json:"id"`
	Name                        string      `json:"name"`
	Source                      *Source     `json:"source,omitempty"`
	Line                        int         `json:"line"`
	Column                      int         `json:"column"`
	EndLine                     int         `json:"endLine,omitempty"`
	EndColumn                   int         `json:"endColumn,omitempty"`
	CanRestart                  bool        `json:"canRestart,omitempty"`
	InstructionPointerReference string      `json:"instructionPointerReference,omitempty"`
	ModuleID                    interface{} `json:"moduleId,omitempty"`
	PresentationHint            string      `json:"presentationHint,omitempty"`
}

// Scope represents a variable scope.
type Scope struct {
	Name               string  `json:"name"`
	PresentationHint   string  `json:"presentationHint,omitempty"`
	VariablesReference int     `json:"variablesReference"`
	NamedVariables     int     `json:"namedVariables,omitempty"`
	IndexedVariables   int     `json:"indexedVariables,omitempty"`
	Expensive          bool    `json:"expensive"`
	Source             *Source `json:"source,omitempty"`
	Line               int     `json:"line,omitempty"`
	Column             int     `json:"column,omitempty"`
	EndLine            int     `json:"endLine,omitempty"`
	EndColumn          int     `json:"endColumn,omitempty"`
}

// Variable represents a variable or field.
type Variable struct {
	Name               string                    `json:"name"`
	Value              string                    `json:"value"`
	Type               string                    `json:"type,omitempty"`
	PresentationHint   *VariablePresentationHint `json:"presentationHint,omitempty"`
	EvaluateName       string                    `json:"evaluateName,omitempty"`
	VariablesReference int                       `json:"variablesReference"`
	NamedVariables     int                       `json:"namedVariables,omitempty"`
	IndexedVariables   int                       `json:"indexedVariables,omitempty"`
	MemoryReference    string                    `json:"memoryReference,omitempty"`
}

// VariablePresentationHint provides rendering hints for variables.
type VariablePresentationHint struct {
	Kind       string   `json:"kind,omitempty"`
	Attributes []string `json:"attributes,omitempty"`
	Visibility string   `json:"visibility,omitempty"`
	Lazy       bool     `json:"lazy,omitempty"`
}

// StoppedEventBody is the body of the stopped event.
type StoppedEventBody struct {
	Reason            string `json:"reason"` // "step", "breakpoint", "exception", "pause", "entry", "goto", "function breakpoint", "data breakpoint", "instruction breakpoint"
	Description       string `json:"description,omitempty"`
	ThreadID          int    `json:"threadId,omitempty"`
	PreserveFocusHint bool   `json:"preserveFocusHint,omitempty"`
	Text              string `json:"text,omitempty"`
	AllThreadsStopped bool   `json:"allThreadsStopped,omitempty"`
	HitBreakpointIds  []int  `json:"hitBreakpointIds,omitempty"`
}

// ContinuedEventBody is the body of the continued event.
type ContinuedEventBody struct {
	ThreadID            int  `json:"threadId"`
	AllThreadsContinued bool `json:"allThreadsContinued,omitempty"`
}

// ExitedEventBody is the body of the exited event.
type ExitedEventBody struct {
	ExitCode int `json:"exitCode"`
}

// TerminatedEventBody is the body of the terminated event.
type TerminatedEventBody struct {
	Restart interface{} `json:"restart,omitempty"`
}

// ThreadEventBody is the body of the thread event.
type ThreadEventBody struct {
	Reason   string `json:"reason"` // "started", "exited"
	ThreadID int    `json:"threadId"`
}

// OutputEventBody is the body of the output event.
type OutputEventBody struct {
	Category string      `json:"category,omitempty"` // "console", "important", "stdout", "stderr", "telemetry"
	Output   string      `json:"output"`
	Group    string      `json:"group,omitempty"` // "start", "startCollapsed", "end"
	Source   *Source     `json:"source,omitempty"`
	Line     int         `json:"line,omitempty"`
	Column   int         `json:"column,omitempty"`
	Data     interface{} `json:"data,omitempty"`
}

// BreakpointEventBody is the body of the breakpoint event.
type BreakpointEventBody struct {
	Reason     string     `json:"reason"` // "changed", "new", "removed"
	Breakpoint Breakpoint `json:"breakpoint"`
}

// ModuleEventBody is the body of the module event.
type ModuleEventBody struct {
	Reason string `json:"reason"` // "new", "changed", "removed"
	Module Module `json:"module"`
}

// Module represents a module (library/dll).
type Module struct {
	ID             interface{} `json:"id"` // int or string
	Name           string      `json:"name"`
	Path           string      `json:"path,omitempty"`
	IsOptimized    bool        `json:"isOptimized,omitempty"`
	IsUserCode     bool        `json:"isUserCode,omitempty"`
	Version        string      `json:"version,omitempty"`
	SymbolStatus   string      `json:"symbolStatus,omitempty"`
	SymbolFilePath string      `json:"symbolFilePath,omitempty"`
	DateTimeStamp  string      `json:"dateTimeStamp,omitempty"`
	AddressRange   string      `json:"addressRange,omitempty"`
}

// LoadedSourceEventBody is the body of the loadedSource event.
type LoadedSourceEventBody struct {
	Reason string `json:"reason"` // "new", "changed", "removed"
	Source Source `json:"source"`
}

// ProcessEventBody is the body of the process event.
type ProcessEventBody struct {
	Name            string `json:"name"`
	SystemProcessID int    `json:"systemProcessId,omitempty"`
	IsLocalProcess  bool   `json:"isLocalProcess,omitempty"`
	StartMethod     string `json:"startMethod,omitempty"` // "launch", "attach", "attachForSuspendedLaunch"
	PointerSize     int    `json:"pointerSize,omitempty"`
}

// CapabilitiesEventBody is the body of the capabilities event.
type CapabilitiesEventBody struct {
	Capabilities Capabilities `json:"capabilities"`
}

// ProgressStartEventBody is the body of the progressStart event.
type ProgressStartEventBody struct {
	ProgressID  string `json:"progressId"`
	Title       string `json:"title"`
	RequestID   int    `json:"requestId,omitempty"`
	Cancellable bool   `json:"cancellable,omitempty"`
	Message     string `json:"message,omitempty"`
	Percentage  int    `json:"percentage,omitempty"`
}

// ProgressUpdateEventBody is the body of the progressUpdate event.
type ProgressUpdateEventBody struct {
	ProgressID string `json:"progressId"`
	Message    string `json:"message,omitempty"`
	Percentage int    `json:"percentage,omitempty"`
}

// ProgressEndEventBody is the body of the progressEnd event.
type ProgressEndEventBody struct {
	ProgressID string `json:"progressId"`
	Message    string `json:"message,omitempty"`
}

// DisconnectArguments are the arguments for disconnect.
type DisconnectArguments struct {
	Restart           bool `json:"restart,omitempty"`
	TerminateDebuggee bool `json:"terminateDebuggee,omitempty"`
	SuspendDebuggee   bool `json:"suspendDebuggee,omitempty"`
}

// TerminateArguments are the arguments for terminate.
type TerminateArguments struct {
	Restart bool `json:"restart,omitempty"`
}

// SetVariableArguments are the arguments for setVariable.
type SetVariableArguments struct {
	VariablesReference int    `json:"variablesReference"`
	Name               string `json:"name"`
	Value              string `json:"value"`
}

// SetVariableResponseBody is the response body for setVariable.
type SetVariableResponseBody struct {
	Value              string `json:"value"`
	Type               string `json:"type,omitempty"`
	VariablesReference int    `json:"variablesReference,omitempty"`
	NamedVariables     int    `json:"namedVariables,omitempty"`
	IndexedVariables   int    `json:"indexedVariables,omitempty"`
}

// SourceArguments are the arguments for source.
type SourceArguments struct {
	Source          *Source `json:"source,omitempty"`
	SourceReference int     `json:"sourceReference"`
}

// SourceResponseBody is the response body for source.
type SourceResponseBody struct {
	Content  string `json:"content"`
	MimeType string `json:"mimeType,omitempty"`
}
