package project

import (
	"context"
	"testing"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
	"github.com/dshills/keystorm/internal/project/filestore"
	"github.com/dshills/keystorm/internal/project/graph"
	"github.com/dshills/keystorm/internal/project/index"
	"github.com/dshills/keystorm/internal/project/workspace"
)

// mockProject implements the Project interface for testing.
type mockProject struct {
	isOpen          bool
	root            string
	roots           []string
	openedFiles     map[string]*filestore.Document
	openFileCalled  bool
	saveFileCalled  bool
	closeFileCalled bool
	err             error
}

func newMockProject() *mockProject {
	return &mockProject{
		openedFiles: make(map[string]*filestore.Document),
	}
}

func (m *mockProject) Open(ctx context.Context, roots ...string) error {
	if m.err != nil {
		return m.err
	}
	m.isOpen = true
	m.roots = roots
	if len(roots) > 0 {
		m.root = roots[0]
	}
	return nil
}

func (m *mockProject) Close(ctx context.Context) error {
	if m.err != nil {
		return m.err
	}
	m.isOpen = false
	return nil
}

func (m *mockProject) IsOpen() bool {
	return m.isOpen
}

func (m *mockProject) Root() string {
	return m.root
}

func (m *mockProject) Roots() []string {
	return m.roots
}

func (m *mockProject) IsInWorkspace(path string) bool {
	return true
}

func (m *mockProject) Workspace() *workspace.Workspace {
	return nil
}

func (m *mockProject) OpenFile(ctx context.Context, path string) (*filestore.Document, error) {
	m.openFileCalled = true
	if m.err != nil {
		return nil, m.err
	}
	doc := &filestore.Document{Path: path, LanguageID: "go"}
	m.openedFiles[path] = doc
	return doc, nil
}

func (m *mockProject) SaveFile(ctx context.Context, path string) error {
	m.saveFileCalled = true
	return m.err
}

func (m *mockProject) SaveFileAs(ctx context.Context, oldPath, newPath string) error {
	return m.err
}

func (m *mockProject) CloseFile(ctx context.Context, path string) error {
	m.closeFileCalled = true
	return m.err
}

func (m *mockProject) CreateFile(ctx context.Context, path string, content []byte) error {
	return m.err
}

func (m *mockProject) DeleteFile(ctx context.Context, path string) error {
	return m.err
}

func (m *mockProject) RenameFile(ctx context.Context, oldPath, newPath string) error {
	return m.err
}

func (m *mockProject) ReloadFile(ctx context.Context, path string) error {
	return m.err
}

func (m *mockProject) CreateDirectory(ctx context.Context, path string) error {
	return m.err
}

func (m *mockProject) DeleteDirectory(ctx context.Context, path string, recursive bool) error {
	return m.err
}

func (m *mockProject) ListDirectory(ctx context.Context, path string) ([]index.FileInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []index.FileInfo{
		{Path: "/test/file1.go", Name: "file1.go", IsDir: false},
		{Path: "/test/dir1", Name: "dir1", IsDir: true},
	}, nil
}

func (m *mockProject) FindFiles(ctx context.Context, query string, opts FindOptions) ([]FileMatch, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []FileMatch{
		{Path: "/test/found.go", Name: "found.go", Score: 1.0},
	}, nil
}

func (m *mockProject) SearchContent(ctx context.Context, query string, opts SearchOptions) ([]ContentMatch, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []ContentMatch{
		{Path: "/test/file.go", Line: 10, Column: 5, Text: "test match"},
	}, nil
}

func (m *mockProject) Graph() graph.Graph {
	return nil
}

func (m *mockProject) RelatedFiles(ctx context.Context, path string) ([]RelatedFile, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []RelatedFile{
		{Path: "/test/related.go", Relationship: "imports", Score: 0.9},
	}, nil
}

func (m *mockProject) OpenDocuments() []*filestore.Document {
	var docs []*filestore.Document
	for _, doc := range m.openedFiles {
		docs = append(docs, doc)
	}
	return docs
}

func (m *mockProject) GetDocument(path string) (*filestore.Document, bool) {
	doc, ok := m.openedFiles[path]
	return doc, ok
}

func (m *mockProject) IsDirty(path string) bool {
	return false
}

func (m *mockProject) DirtyDocuments() []*filestore.Document {
	return nil
}

func (m *mockProject) OnFileChange(handler func(FileChangeEvent)) {}

func (m *mockProject) OnWorkspaceChange(handler func(workspace.ChangeEvent)) {}

func (m *mockProject) IndexStatus() IndexStatus {
	return IndexStatus{Status: "idle", TotalFiles: 100, IndexedFiles: 50}
}

func (m *mockProject) WatcherStatus() WatcherStatus {
	return WatcherStatus{WatchedPaths: 10, TotalEvents: 100}
}

// Helper to create action with args
func actionWithArgs(name string, args map[string]interface{}) input.Action {
	return input.Action{
		Name: name,
		Args: input.ActionArgs{Extra: args},
	}
}

func TestNewHandler(t *testing.T) {
	proj := newMockProject()
	h := NewHandler(proj)

	if h == nil {
		t.Fatal("NewHandler returned nil")
	}

	if h.Namespace() != "project" {
		t.Errorf("Namespace() = %q, want %q", h.Namespace(), "project")
	}

	if h.Priority() != 100 {
		t.Errorf("Priority() = %d, want %d", h.Priority(), 100)
	}
}

func TestHandlerCanHandle(t *testing.T) {
	proj := newMockProject()
	h := NewHandler(proj)

	tests := []struct {
		name string
		want bool
	}{
		{"openFile", true},
		{"project.openFile", true},
		{"saveFile", true},
		{"project.saveFile", true},
		{"unknownAction", false},
		{"project.unknownAction", false},
		{"other.openFile", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.CanHandle(tt.name)
			if got != tt.want {
				t.Errorf("CanHandle(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestHandleOpenFile(t *testing.T) {
	proj := newMockProject()
	h := NewHandler(proj)
	ctx := execctx.New()

	// Test without path
	result := h.HandleAction(actionWithArgs("project.openFile", nil), ctx)
	if result.Status != handler.StatusError {
		t.Errorf("expected error for missing path, got %v", result.Status)
	}

	// Test with valid path
	result = h.HandleAction(actionWithArgs("project.openFile", map[string]interface{}{
		"path": "/test/file.go",
	}), ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	if !proj.openFileCalled {
		t.Error("OpenFile was not called on project")
	}

	// Check data is returned
	doc, ok := result.GetData("document")
	if !ok {
		t.Error("expected document in result data")
	}
	if doc == nil {
		t.Error("document should not be nil")
	}
}

func TestHandleSaveFile(t *testing.T) {
	proj := newMockProject()
	h := NewHandler(proj)
	ctx := execctx.New()

	// Test without path
	result := h.HandleAction(actionWithArgs("project.saveFile", nil), ctx)
	if result.Status != handler.StatusError {
		t.Errorf("expected error for missing path, got %v", result.Status)
	}

	// Test with valid path
	result = h.HandleAction(actionWithArgs("project.saveFile", map[string]interface{}{
		"path": "/test/file.go",
	}), ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	if !proj.saveFileCalled {
		t.Error("SaveFile was not called on project")
	}
}

func TestHandleCloseFile(t *testing.T) {
	proj := newMockProject()
	h := NewHandler(proj)
	ctx := execctx.New()

	result := h.HandleAction(actionWithArgs("project.closeFile", map[string]interface{}{
		"path": "/test/file.go",
	}), ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	if !proj.closeFileCalled {
		t.Error("CloseFile was not called on project")
	}
}

func TestHandleOpenAndClose(t *testing.T) {
	proj := newMockProject()
	h := NewHandler(proj)
	ctx := execctx.New()

	// Test open with root
	result := h.HandleAction(actionWithArgs("project.open", map[string]interface{}{
		"root": "/workspace",
	}), ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("open: expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	if !proj.isOpen {
		t.Error("project should be open")
	}

	// Test close
	result = h.HandleAction(actionWithArgs("project.close", nil), ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("close: expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	if proj.isOpen {
		t.Error("project should be closed")
	}
}

func TestHandleOpenWithRoots(t *testing.T) {
	proj := newMockProject()
	h := NewHandler(proj)
	ctx := execctx.New()

	// Test open with multiple roots
	result := h.HandleAction(actionWithArgs("project.open", map[string]interface{}{
		"roots": []any{"/workspace1", "/workspace2"},
	}), ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	if len(proj.roots) != 2 {
		t.Errorf("expected 2 roots, got %d", len(proj.roots))
	}
}

func TestHandleOpenRequiresRoot(t *testing.T) {
	proj := newMockProject()
	h := NewHandler(proj)
	ctx := execctx.New()

	result := h.HandleAction(actionWithArgs("project.open", nil), ctx)

	if result.Status != handler.StatusError {
		t.Errorf("expected error for missing root, got %v", result.Status)
	}
}

func TestHandleListDirectory(t *testing.T) {
	proj := newMockProject()
	h := NewHandler(proj)
	ctx := execctx.New()

	result := h.HandleAction(actionWithArgs("project.listDirectory", map[string]interface{}{
		"path": "/test",
	}), ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	entries, ok := result.GetData("entries")
	if !ok {
		t.Error("expected entries in result data")
	}

	entriesSlice, ok := entries.([]index.FileInfo)
	if !ok {
		t.Error("entries should be []index.FileInfo")
	}

	if len(entriesSlice) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entriesSlice))
	}
}

func TestHandleFindFiles(t *testing.T) {
	proj := newMockProject()
	h := NewHandler(proj)
	ctx := execctx.New()

	result := h.HandleAction(actionWithArgs("project.findFiles", map[string]interface{}{
		"query": "test",
	}), ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	matches, ok := result.GetData("matches")
	if !ok {
		t.Error("expected matches in result data")
	}

	matchesSlice, ok := matches.([]FileMatch)
	if !ok {
		t.Error("matches should be []FileMatch")
	}

	if len(matchesSlice) != 1 {
		t.Errorf("expected 1 match, got %d", len(matchesSlice))
	}
}

func TestHandleSearchContent(t *testing.T) {
	proj := newMockProject()
	h := NewHandler(proj)
	ctx := execctx.New()

	result := h.HandleAction(actionWithArgs("project.searchContent", map[string]interface{}{
		"query":         "test",
		"caseSensitive": true,
	}), ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	matches, ok := result.GetData("matches")
	if !ok {
		t.Error("expected matches in result data")
	}

	matchesSlice, ok := matches.([]ContentMatch)
	if !ok {
		t.Error("matches should be []ContentMatch")
	}

	if len(matchesSlice) != 1 {
		t.Errorf("expected 1 match, got %d", len(matchesSlice))
	}
}

func TestHandleRelatedFiles(t *testing.T) {
	proj := newMockProject()
	h := NewHandler(proj)
	ctx := execctx.New()

	result := h.HandleAction(actionWithArgs("project.relatedFiles", map[string]interface{}{
		"path": "/test/file.go",
	}), ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	related, ok := result.GetData("related")
	if !ok {
		t.Error("expected related in result data")
	}

	relatedSlice, ok := related.([]RelatedFile)
	if !ok {
		t.Error("related should be []RelatedFile")
	}

	if len(relatedSlice) != 1 {
		t.Errorf("expected 1 related file, got %d", len(relatedSlice))
	}
}

func TestHandleIndexStatus(t *testing.T) {
	proj := newMockProject()
	h := NewHandler(proj)
	ctx := execctx.New()

	result := h.HandleAction(actionWithArgs("project.indexStatus", nil), ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	status, ok := result.GetData("status")
	if !ok {
		t.Error("expected status in result data")
	}

	indexStatus, ok := status.(IndexStatus)
	if !ok {
		t.Error("status should be IndexStatus")
	}

	if indexStatus.TotalFiles != 100 {
		t.Errorf("expected TotalFiles=100, got %d", indexStatus.TotalFiles)
	}
}

func TestHandleWatcherStatus(t *testing.T) {
	proj := newMockProject()
	h := NewHandler(proj)
	ctx := execctx.New()

	result := h.HandleAction(actionWithArgs("project.watcherStatus", nil), ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	status, ok := result.GetData("status")
	if !ok {
		t.Error("expected status in result data")
	}

	watcherStatus, ok := status.(WatcherStatus)
	if !ok {
		t.Error("status should be WatcherStatus")
	}

	if watcherStatus.WatchedPaths != 10 {
		t.Errorf("expected WatchedPaths=10, got %d", watcherStatus.WatchedPaths)
	}
}

func TestHandleUnknownAction(t *testing.T) {
	proj := newMockProject()
	h := NewHandler(proj)
	ctx := execctx.New()

	result := h.HandleAction(actionWithArgs("project.unknownAction", nil), ctx)

	if result.Status != handler.StatusError {
		t.Errorf("expected StatusError for unknown action, got %v", result.Status)
	}
}

func TestActionNames(t *testing.T) {
	// Verify action name constants are correctly defined
	names := []string{
		ActionOpenFile,
		ActionSaveFile,
		ActionSaveFileAs,
		ActionCloseFile,
		ActionReloadFile,
		ActionCreateFile,
		ActionDeleteFile,
		ActionRenameFile,
		ActionCreateDir,
		ActionDeleteDir,
		ActionListDir,
		ActionFindFiles,
		ActionSearchContent,
		ActionRelatedFiles,
		ActionOpen,
		ActionClose,
		ActionIndexStatus,
		ActionWatcherStatus,
		ActionOpenDocuments,
		ActionDirtyDocuments,
	}

	proj := newMockProject()
	h := NewHandler(proj)

	for _, name := range names {
		if !h.CanHandle(name) {
			t.Errorf("handler should be able to handle %q", name)
		}
	}
}
