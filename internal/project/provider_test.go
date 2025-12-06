package project

import (
	"context"
	"testing"

	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/plugin/security"
	"github.com/dshills/keystorm/internal/project/filestore"
	"github.com/dshills/keystorm/internal/project/index"
)

// mockProvider implements the Provider interface for testing.
type mockProvider struct {
	isOpen    bool
	roots     []string
	documents map[string]*filestore.Document
	err       error
}

func newMockProvider() *mockProvider {
	return &mockProvider{
		documents: make(map[string]*filestore.Document),
	}
}

func (m *mockProvider) IsOpen() bool {
	return m.isOpen
}

func (m *mockProvider) Roots() []string {
	return m.roots
}

func (m *mockProvider) OpenFile(ctx context.Context, path string) (*filestore.Document, error) {
	if m.err != nil {
		return nil, m.err
	}
	doc := &filestore.Document{Path: path, LanguageID: "go"}
	m.documents[path] = doc
	return doc, nil
}

func (m *mockProvider) SaveFile(ctx context.Context, path string) error {
	return m.err
}

func (m *mockProvider) CloseFile(ctx context.Context, path string) error {
	delete(m.documents, path)
	return m.err
}

func (m *mockProvider) CreateFile(ctx context.Context, path string, content []byte) error {
	return m.err
}

func (m *mockProvider) DeleteFile(ctx context.Context, path string) error {
	return m.err
}

func (m *mockProvider) ListDirectory(ctx context.Context, path string) ([]index.FileInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []index.FileInfo{
		{Path: "/test/file1.go", Name: "file1.go", IsDir: false, Size: 100},
		{Path: "/test/dir1", Name: "dir1", IsDir: true, Size: 0},
	}, nil
}

func (m *mockProvider) FindFiles(ctx context.Context, pattern string, opts FindOptions) ([]FileMatch, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []FileMatch{
		{Path: "/test/found.go", Name: "found.go", Score: 1.0},
	}, nil
}

func (m *mockProvider) SearchContent(ctx context.Context, query string, opts SearchOptions) ([]ContentMatch, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []ContentMatch{
		{Path: "/test/file.go", Line: 10, Column: 5, Text: "test match"},
	}, nil
}

func (m *mockProvider) RelatedFiles(ctx context.Context, path string) ([]RelatedFile, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []RelatedFile{
		{Path: "/test/related.go", Relationship: "imports", Score: 0.9},
	}, nil
}

func (m *mockProvider) OpenDocuments() []*filestore.Document {
	var docs []*filestore.Document
	for _, doc := range m.documents {
		docs = append(docs, doc)
	}
	return docs
}

func (m *mockProvider) DirtyDocuments() []*filestore.Document {
	return nil
}

func (m *mockProvider) IndexStatus() IndexStatus {
	return IndexStatus{Status: "idle", TotalFiles: 100, IndexedFiles: 50}
}

func (m *mockProvider) WatcherStatus() WatcherStatus {
	return WatcherStatus{WatchedPaths: 10, TotalEvents: 100}
}

func TestNewModule(t *testing.T) {
	provider := newMockProvider()
	mod := NewModule(provider)

	if mod == nil {
		t.Fatal("NewModule returned nil")
	}

	if mod.Name() != "project" {
		t.Errorf("Name() = %q, want %q", mod.Name(), "project")
	}

	if mod.RequiredCapability() != security.CapabilityProject {
		t.Errorf("RequiredCapability() = %v, want %v", mod.RequiredCapability(), security.CapabilityProject)
	}
}

func TestModuleRegister(t *testing.T) {
	provider := newMockProvider()
	mod := NewModule(provider)

	L := lua.NewState()
	defer L.Close()

	err := mod.Register(L)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Check that _ks_project global was set
	global := L.GetGlobal("_ks_project")
	if global == lua.LNil {
		t.Fatal("_ks_project global not set")
	}

	tbl, ok := global.(*lua.LTable)
	if !ok {
		t.Fatal("_ks_project should be a table")
	}

	// Check some functions exist
	funcs := []string{"is_open", "roots", "open_file", "save_file", "find_files", "search_content"}
	for _, fn := range funcs {
		val := tbl.RawGetString(fn)
		if val == lua.LNil {
			t.Errorf("function %q not found in module", fn)
		}
	}
}

func TestLuaIsOpen(t *testing.T) {
	provider := newMockProvider()
	provider.isOpen = true

	mod := NewModule(provider)

	L := lua.NewState()
	defer L.Close()

	_ = mod.Register(L)

	// Call is_open()
	err := L.DoString(`
		local p = _ks_project
		result = p.is_open()
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	result := L.GetGlobal("result")
	if result != lua.LTrue {
		t.Errorf("is_open() = %v, want true", result)
	}
}

func TestLuaRoots(t *testing.T) {
	provider := newMockProvider()
	provider.roots = []string{"/workspace1", "/workspace2"}

	mod := NewModule(provider)

	L := lua.NewState()
	defer L.Close()

	_ = mod.Register(L)

	err := L.DoString(`
		local p = _ks_project
		roots = p.roots()
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	roots := L.GetGlobal("roots")
	tbl, ok := roots.(*lua.LTable)
	if !ok {
		t.Fatal("roots() should return a table")
	}

	if tbl.Len() != 2 {
		t.Errorf("roots() returned %d items, want 2", tbl.Len())
	}
}

func TestLuaOpenFile(t *testing.T) {
	provider := newMockProvider()
	mod := NewModule(provider)

	L := lua.NewState()
	defer L.Close()

	_ = mod.Register(L)

	err := L.DoString(`
		local p = _ks_project
		doc, err = p.open_file("/test/file.go")
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	doc := L.GetGlobal("doc")
	if doc == lua.LNil {
		t.Fatal("open_file should return document")
	}

	errVal := L.GetGlobal("err")
	if errVal != lua.LNil {
		t.Errorf("open_file should not return error, got: %v", errVal)
	}

	docTbl, ok := doc.(*lua.LTable)
	if !ok {
		t.Fatal("document should be a table")
	}

	path := docTbl.RawGetString("path")
	if path.String() != "/test/file.go" {
		t.Errorf("doc.path = %q, want %q", path.String(), "/test/file.go")
	}
}

func TestLuaListDirectory(t *testing.T) {
	provider := newMockProvider()
	mod := NewModule(provider)

	L := lua.NewState()
	defer L.Close()

	_ = mod.Register(L)

	err := L.DoString(`
		local p = _ks_project
		entries, err = p.list_directory("/test")
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	entries := L.GetGlobal("entries")
	tbl, ok := entries.(*lua.LTable)
	if !ok {
		t.Fatal("list_directory should return a table")
	}

	if tbl.Len() != 2 {
		t.Errorf("list_directory returned %d items, want 2", tbl.Len())
	}
}

func TestLuaFindFiles(t *testing.T) {
	provider := newMockProvider()
	mod := NewModule(provider)

	L := lua.NewState()
	defer L.Close()

	_ = mod.Register(L)

	err := L.DoString(`
		local p = _ks_project
		matches, err = p.find_files("test")
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	matches := L.GetGlobal("matches")
	tbl, ok := matches.(*lua.LTable)
	if !ok {
		t.Fatal("find_files should return a table")
	}

	if tbl.Len() != 1 {
		t.Errorf("find_files returned %d items, want 1", tbl.Len())
	}
}

func TestLuaSearchContent(t *testing.T) {
	provider := newMockProvider()
	mod := NewModule(provider)

	L := lua.NewState()
	defer L.Close()

	_ = mod.Register(L)

	err := L.DoString(`
		local p = _ks_project
		matches, err = p.search_content("test")
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	matches := L.GetGlobal("matches")
	tbl, ok := matches.(*lua.LTable)
	if !ok {
		t.Fatal("search_content should return a table")
	}

	if tbl.Len() != 1 {
		t.Errorf("search_content returned %d items, want 1", tbl.Len())
	}
}

func TestLuaSearchContentWithOptions(t *testing.T) {
	provider := newMockProvider()
	mod := NewModule(provider)

	L := lua.NewState()
	defer L.Close()

	_ = mod.Register(L)

	err := L.DoString(`
		local p = _ks_project
		matches, err = p.search_content("test", {
			case_sensitive = true,
			whole_word = true,
			max_results = 50
		})
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	matches := L.GetGlobal("matches")
	if matches == lua.LNil {
		t.Fatal("search_content should return matches")
	}
}

func TestLuaRelatedFiles(t *testing.T) {
	provider := newMockProvider()
	mod := NewModule(provider)

	L := lua.NewState()
	defer L.Close()

	_ = mod.Register(L)

	err := L.DoString(`
		local p = _ks_project
		related, err = p.related_files("/test/file.go")
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	related := L.GetGlobal("related")
	tbl, ok := related.(*lua.LTable)
	if !ok {
		t.Fatal("related_files should return a table")
	}

	if tbl.Len() != 1 {
		t.Errorf("related_files returned %d items, want 1", tbl.Len())
	}
}

func TestLuaIndexStatus(t *testing.T) {
	provider := newMockProvider()
	mod := NewModule(provider)

	L := lua.NewState()
	defer L.Close()

	_ = mod.Register(L)

	err := L.DoString(`
		local p = _ks_project
		status = p.index_status()
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	status := L.GetGlobal("status")
	tbl, ok := status.(*lua.LTable)
	if !ok {
		t.Fatal("index_status should return a table")
	}

	totalFiles := tbl.RawGetString("total_files")
	if totalFiles != lua.LNumber(100) {
		t.Errorf("status.total_files = %v, want 100", totalFiles)
	}
}

func TestLuaWatcherStatus(t *testing.T) {
	provider := newMockProvider()
	mod := NewModule(provider)

	L := lua.NewState()
	defer L.Close()

	_ = mod.Register(L)

	err := L.DoString(`
		local p = _ks_project
		status = p.watcher_status()
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	status := L.GetGlobal("status")
	tbl, ok := status.(*lua.LTable)
	if !ok {
		t.Fatal("watcher_status should return a table")
	}

	watchedPaths := tbl.RawGetString("watched_paths")
	if watchedPaths != lua.LNumber(10) {
		t.Errorf("status.watched_paths = %v, want 10", watchedPaths)
	}
}

func TestAdapter(t *testing.T) {
	// Test that Adapter properly implements Provider
	proj := newMockProject()
	adapter := NewAdapter(proj)

	// Verify adapter methods delegate to project
	proj.isOpen = true
	if !adapter.IsOpen() {
		t.Error("Adapter.IsOpen() should return project's IsOpen()")
	}

	proj.roots = []string{"/test"}
	roots := adapter.Roots()
	if len(roots) != 1 || roots[0] != "/test" {
		t.Error("Adapter.Roots() should return project's Roots()")
	}

	// Test file operations
	doc, err := adapter.OpenFile(context.Background(), "/test/file.go")
	if err != nil {
		t.Errorf("Adapter.OpenFile() error: %v", err)
	}
	if doc == nil {
		t.Error("Adapter.OpenFile() should return document")
	}

	// Test status operations
	indexStatus := adapter.IndexStatus()
	if indexStatus.TotalFiles != 100 {
		t.Errorf("Adapter.IndexStatus().TotalFiles = %d, want 100", indexStatus.TotalFiles)
	}

	watcherStatus := adapter.WatcherStatus()
	if watcherStatus.WatchedPaths != 10 {
		t.Errorf("Adapter.WatcherStatus().WatchedPaths = %d, want 10", watcherStatus.WatchedPaths)
	}
}

func TestAdapterImplementsProvider(t *testing.T) {
	proj := newMockProject()
	adapter := NewAdapter(proj)

	// Verify it satisfies the Provider interface
	var _ Provider = adapter
}
