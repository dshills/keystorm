// Package project provides the handler for dispatcher integration.
package project

import (
	"context"
	"strings"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
)

// Handler provides project operations as dispatcher actions.
// It implements the NamespaceHandler interface to handle all "project.*" actions.
type Handler struct {
	project  Project
	actions  map[string]func(action input.Action, ctx *execctx.ExecutionContext) handler.Result
	priority int
}

// NewHandler creates a new project handler.
func NewHandler(proj Project) *Handler {
	h := &Handler{
		project:  proj,
		actions:  make(map[string]func(action input.Action, ctx *execctx.ExecutionContext) handler.Result),
		priority: 100, // Plugin-level priority
	}
	h.registerActions()
	return h
}

// Namespace returns the namespace prefix for this handler.
func (h *Handler) Namespace() string {
	return "project"
}

// CanHandle returns true if this handler can process the action.
func (h *Handler) CanHandle(actionName string) bool {
	// Strip namespace prefix if present
	name := actionName
	if strings.HasPrefix(actionName, "project.") {
		name = strings.TrimPrefix(actionName, "project.")
	}
	_, ok := h.actions[name]
	return ok
}

// HandleAction handles an action within the project namespace.
func (h *Handler) HandleAction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	// Strip namespace prefix if present
	name := action.Name
	if strings.HasPrefix(action.Name, "project.") {
		name = strings.TrimPrefix(action.Name, "project.")
	}

	fn, ok := h.actions[name]
	if !ok {
		return handler.Errorf("unknown project action: %s", action.Name)
	}
	return fn(action, ctx)
}

// Priority returns the handler priority.
func (h *Handler) Priority() int {
	return h.priority
}

// registerActions registers all project action handlers.
func (h *Handler) registerActions() {
	// File operations
	h.actions["openFile"] = h.handleOpenFile
	h.actions["saveFile"] = h.handleSaveFile
	h.actions["saveFileAs"] = h.handleSaveFileAs
	h.actions["closeFile"] = h.handleCloseFile
	h.actions["reloadFile"] = h.handleReloadFile
	h.actions["createFile"] = h.handleCreateFile
	h.actions["deleteFile"] = h.handleDeleteFile
	h.actions["renameFile"] = h.handleRenameFile

	// Directory operations
	h.actions["createDirectory"] = h.handleCreateDirectory
	h.actions["deleteDirectory"] = h.handleDeleteDirectory
	h.actions["listDirectory"] = h.handleListDirectory

	// Search operations
	h.actions["findFiles"] = h.handleFindFiles
	h.actions["searchContent"] = h.handleSearchContent

	// Project graph operations
	h.actions["relatedFiles"] = h.handleRelatedFiles

	// Workspace operations
	h.actions["open"] = h.handleOpen
	h.actions["close"] = h.handleClose

	// Status operations
	h.actions["indexStatus"] = h.handleIndexStatus
	h.actions["watcherStatus"] = h.handleWatcherStatus

	// Document operations
	h.actions["openDocuments"] = h.handleOpenDocuments
	h.actions["dirtyDocuments"] = h.handleDirtyDocuments
}

// getStringArg gets a string argument from action args.Extra.
func getStringArg(action input.Action, key string) string {
	return action.Args.GetString(key)
}

// getBytesArg gets a []byte argument from action args.Extra.
func getBytesArg(action input.Action, key string) []byte {
	if v, ok := action.Args.Get(key); ok {
		switch val := v.(type) {
		case []byte:
			return val
		case string:
			return []byte(val)
		}
	}
	return nil
}

// getBoolArg gets a bool argument from action args.Extra.
func getBoolArg(action input.Action, key string, defaultVal bool) bool {
	if v, ok := action.Args.Get(key); ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultVal
}

// getIntArg gets an int argument from action args.Extra.
func getIntArg(action input.Action, key string, defaultVal int) int {
	if v, ok := action.Args.Get(key); ok {
		switch val := v.(type) {
		case int:
			return val
		case int64:
			return int(val)
		case float64:
			return int(val)
		}
	}
	return defaultVal
}

// File operation handlers

func (h *Handler) handleOpenFile(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	path := getStringArg(action, "path")
	if path == "" {
		return handler.Errorf("openFile: path is required")
	}

	doc, err := h.project.OpenFile(context.Background(), path)
	if err != nil {
		return handler.Errorf("openFile: %v", err)
	}

	return handler.SuccessWithData("document", doc).WithMessage("File opened: " + path)
}

func (h *Handler) handleSaveFile(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	path := getStringArg(action, "path")
	if path == "" {
		return handler.Errorf("saveFile: path is required")
	}

	err := h.project.SaveFile(context.Background(), path)
	if err != nil {
		return handler.Errorf("saveFile: %v", err)
	}

	return handler.SuccessWithMessage("File saved: " + path)
}

func (h *Handler) handleSaveFileAs(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	oldPath := getStringArg(action, "oldPath")
	newPath := getStringArg(action, "newPath")
	if oldPath == "" || newPath == "" {
		return handler.Errorf("saveFileAs: oldPath and newPath are required")
	}

	err := h.project.SaveFileAs(context.Background(), oldPath, newPath)
	if err != nil {
		return handler.Errorf("saveFileAs: %v", err)
	}

	return handler.SuccessWithMessage("File saved as: " + newPath)
}

func (h *Handler) handleCloseFile(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	path := getStringArg(action, "path")
	if path == "" {
		return handler.Errorf("closeFile: path is required")
	}

	err := h.project.CloseFile(context.Background(), path)
	if err != nil {
		return handler.Errorf("closeFile: %v", err)
	}

	return handler.SuccessWithMessage("File closed: " + path)
}

func (h *Handler) handleReloadFile(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	path := getStringArg(action, "path")
	if path == "" {
		return handler.Errorf("reloadFile: path is required")
	}

	err := h.project.ReloadFile(context.Background(), path)
	if err != nil {
		return handler.Errorf("reloadFile: %v", err)
	}

	return handler.SuccessWithMessage("File reloaded: " + path)
}

func (h *Handler) handleCreateFile(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	path := getStringArg(action, "path")
	if path == "" {
		return handler.Errorf("createFile: path is required")
	}

	content := getBytesArg(action, "content")

	err := h.project.CreateFile(context.Background(), path, content)
	if err != nil {
		return handler.Errorf("createFile: %v", err)
	}

	return handler.SuccessWithMessage("File created: " + path)
}

func (h *Handler) handleDeleteFile(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	path := getStringArg(action, "path")
	if path == "" {
		return handler.Errorf("deleteFile: path is required")
	}

	err := h.project.DeleteFile(context.Background(), path)
	if err != nil {
		return handler.Errorf("deleteFile: %v", err)
	}

	return handler.SuccessWithMessage("File deleted: " + path)
}

func (h *Handler) handleRenameFile(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	oldPath := getStringArg(action, "oldPath")
	newPath := getStringArg(action, "newPath")
	if oldPath == "" || newPath == "" {
		return handler.Errorf("renameFile: oldPath and newPath are required")
	}

	err := h.project.RenameFile(context.Background(), oldPath, newPath)
	if err != nil {
		return handler.Errorf("renameFile: %v", err)
	}

	return handler.SuccessWithMessage("File renamed: " + oldPath + " -> " + newPath)
}

// Directory operation handlers

func (h *Handler) handleCreateDirectory(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	path := getStringArg(action, "path")
	if path == "" {
		return handler.Errorf("createDirectory: path is required")
	}

	err := h.project.CreateDirectory(context.Background(), path)
	if err != nil {
		return handler.Errorf("createDirectory: %v", err)
	}

	return handler.SuccessWithMessage("Directory created: " + path)
}

func (h *Handler) handleDeleteDirectory(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	path := getStringArg(action, "path")
	if path == "" {
		return handler.Errorf("deleteDirectory: path is required")
	}

	recursive := getBoolArg(action, "recursive", false)

	err := h.project.DeleteDirectory(context.Background(), path, recursive)
	if err != nil {
		return handler.Errorf("deleteDirectory: %v", err)
	}

	return handler.SuccessWithMessage("Directory deleted: " + path)
}

func (h *Handler) handleListDirectory(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	path := getStringArg(action, "path")
	if path == "" {
		return handler.Errorf("listDirectory: path is required")
	}

	entries, err := h.project.ListDirectory(context.Background(), path)
	if err != nil {
		return handler.Errorf("listDirectory: %v", err)
	}

	return handler.SuccessWithData("entries", entries).WithMessage("Listed directory: " + path)
}

// Search operation handlers

func (h *Handler) handleFindFiles(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	query := getStringArg(action, "query")
	if query == "" {
		return handler.Errorf("findFiles: query is required")
	}

	opts := FindOptions{
		MaxResults: getIntArg(action, "maxResults", 100),
	}

	matches, err := h.project.FindFiles(context.Background(), query, opts)
	if err != nil {
		return handler.Errorf("findFiles: %v", err)
	}

	return handler.SuccessWithData("matches", matches).WithMessage("Found files")
}

func (h *Handler) handleSearchContent(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	query := getStringArg(action, "query")
	if query == "" {
		return handler.Errorf("searchContent: query is required")
	}

	opts := SearchOptions{
		MaxResults:    getIntArg(action, "maxResults", 100),
		CaseSensitive: getBoolArg(action, "caseSensitive", false),
		WholeWord:     getBoolArg(action, "wholeWord", false),
		UseRegex:      getBoolArg(action, "useRegex", false),
	}

	matches, err := h.project.SearchContent(context.Background(), query, opts)
	if err != nil {
		return handler.Errorf("searchContent: %v", err)
	}

	return handler.SuccessWithData("matches", matches).WithMessage("Search completed")
}

// Project graph handlers

func (h *Handler) handleRelatedFiles(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	path := getStringArg(action, "path")
	if path == "" {
		return handler.Errorf("relatedFiles: path is required")
	}

	related, err := h.project.RelatedFiles(context.Background(), path)
	if err != nil {
		return handler.Errorf("relatedFiles: %v", err)
	}

	return handler.SuccessWithData("related", related).WithMessage("Found related files")
}

// Workspace operation handlers

func (h *Handler) handleOpen(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	// Get roots from args - can be single root or multiple
	var roots []string
	if root := getStringArg(action, "root"); root != "" {
		roots = append(roots, root)
	}
	if v, ok := action.Args.Get("roots"); ok {
		if rs, ok := v.([]string); ok {
			roots = append(roots, rs...)
		}
		if rs, ok := v.([]any); ok {
			for _, r := range rs {
				if s, ok := r.(string); ok {
					roots = append(roots, s)
				}
			}
		}
	}

	if len(roots) == 0 {
		return handler.Errorf("open: at least one root is required")
	}

	err := h.project.Open(context.Background(), roots...)
	if err != nil {
		return handler.Errorf("open: %v", err)
	}

	return handler.SuccessWithMessage("Project opened")
}

func (h *Handler) handleClose(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	err := h.project.Close(context.Background())
	if err != nil {
		return handler.Errorf("close: %v", err)
	}

	return handler.SuccessWithMessage("Project closed")
}

// Status operation handlers

func (h *Handler) handleIndexStatus(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	status := h.project.IndexStatus()
	return handler.SuccessWithData("status", status).WithMessage("Index status retrieved")
}

func (h *Handler) handleWatcherStatus(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	status := h.project.WatcherStatus()
	return handler.SuccessWithData("status", status).WithMessage("Watcher status retrieved")
}

// Document operation handlers

func (h *Handler) handleOpenDocuments(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	docs := h.project.OpenDocuments()
	return handler.SuccessWithData("documents", docs).WithMessage("Open documents retrieved")
}

func (h *Handler) handleDirtyDocuments(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	docs := h.project.DirtyDocuments()
	return handler.SuccessWithData("documents", docs).WithMessage("Dirty documents retrieved")
}

// Action names for external use
const (
	ActionOpenFile       = "project.openFile"
	ActionSaveFile       = "project.saveFile"
	ActionSaveFileAs     = "project.saveFileAs"
	ActionCloseFile      = "project.closeFile"
	ActionReloadFile     = "project.reloadFile"
	ActionCreateFile     = "project.createFile"
	ActionDeleteFile     = "project.deleteFile"
	ActionRenameFile     = "project.renameFile"
	ActionCreateDir      = "project.createDirectory"
	ActionDeleteDir      = "project.deleteDirectory"
	ActionListDir        = "project.listDirectory"
	ActionFindFiles      = "project.findFiles"
	ActionSearchContent  = "project.searchContent"
	ActionRelatedFiles   = "project.relatedFiles"
	ActionOpen           = "project.open"
	ActionClose          = "project.close"
	ActionIndexStatus    = "project.indexStatus"
	ActionWatcherStatus  = "project.watcherStatus"
	ActionOpenDocuments  = "project.openDocuments"
	ActionDirtyDocuments = "project.dirtyDocuments"
)
